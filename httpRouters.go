package main

import (
	"cmp"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.goblog.app/app/pkgs/bodylimit"
)

// registerIndexRoutes registers the standard triplet of routes for index pages:
// base path, feed path, and pagination path.
func registerIndexRoutes(r chi.Router, path string, handler http.HandlerFunc) {
	r.Get(path, handler)
	r.Get(path+feedPath, handler)
	r.Get(path+paginationPath, handler)
}

// Login
func (a *goBlog) loginRouter(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(a.authMiddleware)
		r.Get(loginPath, serveLogin)
		r.Get(logoutPath, a.serveLogout)
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(webAuthnBasePath+"registration/begin", a.beginWebAuthnRegistration)
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(webAuthnBasePath+"registration/finish", a.finishWebAuthnRegistration)
	})
	r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(webAuthnBasePath+"login/begin", a.beginWebAuthnLogin)
	r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(webAuthnBasePath+"login/finish", a.finishWebAuthnLogin)
}

// Micropub
func (a *goBlog) micropubRouter(r chi.Router) {
	r.Use(a.checkIndieAuth)
	r.Handle("/", a.getMicropubImplementation().getHandler())
	r.Handle(micropubMediaSubPath, a.getMicropubImplementation().getMediaHandler())
}

// IndieAuth
func (a *goBlog) indieAuthRouter(r chi.Router) {
	r.Route(indieAuthPath, func(r chi.Router) {
		r.Get("/", a.indieAuthRequest)
		r.With(a.authMiddleware).Post("/accept", a.indieAuthAccept)
		r.With(bodylimit.BodyLimit(100*bodylimit.KB)).Post("/", a.indieAuthVerificationAuth)
		r.With(bodylimit.BodyLimit(100*bodylimit.KB)).Post(indieAuthTokenSubpath, a.indieAuthVerificationToken)
		r.Get(indieAuthTokenSubpath, a.indieAuthTokenVerification)
		r.With(bodylimit.BodyLimit(100*bodylimit.KB)).Post(indieAuthTokenRevocationSubpath, a.indieAuthTokenRevokation)
	})
	r.With(cacheLoggedIn, a.cacheMiddleware).Get(indieAuthMetadataPath, a.indieAuthMetadata)
}

// ActivityPub
func (a *goBlog) activityPubRouter(r chi.Router) {
	if a.isPrivate() {
		// Private mode, no ActivityPub
		return
	}
	if ap := a.cfg.ActivityPub; ap != nil && ap.Enabled {
		r.Route(activityPubBasePath, func(r chi.Router) {
			r.With(bodylimit.BodyLimit(10*bodylimit.MB)).Post("/inbox/{blog}", a.apHandleInbox)
			r.With(a.checkActivityStreamsRequest).Get("/followers/{blog}", a.apShowFollowers)
			r.With(a.cacheMiddleware).Get("/remote_follow/{blog}", a.apRemoteFollow)
			r.With(bodylimit.BodyLimit(100*bodylimit.KB)).Post("/remote_follow/{blog}", a.apRemoteFollow)
		})
		r.Group(func(r chi.Router) {
			r.Use(cacheLoggedIn, a.cacheMiddleware)
			r.Get("/.well-known/webfinger", a.apHandleWebfinger)
			r.Get("/.well-known/host-meta", handleWellKnownHostMeta)
			r.Get("/.well-known/nodeinfo", a.serveNodeInfoDiscover)
			r.Get("/nodeinfo", a.serveNodeInfo)
		})
	}
}

// Webmentions
func (a *goBlog) webmentionsRouter(r chi.Router) {
	if wm := a.cfg.Webmention; wm != nil && wm.DisableReceiving {
		// Disabled
		return
	}
	// Endpoint
	r.With(bodylimit.BodyLimit(10*bodylimit.KB)).Post("/", a.handleWebmention)
	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(a.authMiddleware)
		r.Get("/", a.webmentionAdmin)
		r.Get(paginationPath, a.webmentionAdmin)
		r.With(bodylimit.BodyLimit(100*bodylimit.KB)).Post("/{action:(delete|approve|reverify)}", a.webmentionAdminAction)
	})
}

// Notifications
func (a *goBlog) notificationsRouter(r chi.Router) {
	r.Use(a.authMiddleware)
	r.Get("/", a.notificationsAdmin)
	r.Get(paginationPath, a.notificationsAdmin)
	r.With(bodylimit.BodyLimit(10*bodylimit.KB)).Post("/delete", a.notificationsAdminDelete)
}

// Assets
func (a *goBlog) assetsRouter(r chi.Router) {
	for _, path := range a.allAssetPaths() {
		r.Get(path, a.serveAsset)
	}
}

// Static files
func (a *goBlog) staticFilesRouter(r chi.Router) {
	r.Use(a.privateModeHandler)
	for _, path := range allStaticPaths() {
		r.Get(path, a.serveStaticFile)
	}
}

// Media files
func (a *goBlog) mediaFilesRouter(r chi.Router) {
	r.Use(a.privateModeHandler)
	r.Get(mediaFileRoute, a.serveMediaFile)
}

// Profile image
func (a *goBlog) profileImageRouter(r chi.Router) {
	r.Use(keepSelectedQueryParams("s", "q"), cacheLoggedIn, a.cacheMiddleware, noIndexHeader)
	r.Get(profileImagePathJPEG, a.serveProfileImage(profileImageFormatJPEG))
	r.Get(profileImagePathPNG, a.serveProfileImage(profileImageFormatPNG))
}

// Various other routes
func (a *goBlog) otherRoutesRouter(r chi.Router) {
	r.Use(a.privateModeHandler)

	// Leaflet
	r.With(noIndexHeader).Get("/tiles/{s}/{z}/{x}/{y}.png", a.proxyTiles())
	r.With(cacheLoggedIn, a.cacheMiddleware, noIndexHeader).HandleFunc("/leaflet/*", a.serveFs(leafletFiles, "/-/"))

	// Hlsjs
	r.With(cacheLoggedIn, a.cacheMiddleware, noIndexHeader).HandleFunc("/hlsjs/*", a.serveFs(hlsjsFiles, "/-/"))

	// Reactions
	if a.anyReactionsEnabled() {
		r.Get("/reactions", a.getReactions)
		r.With(bodylimit.BodyLimit(10*bodylimit.KB)).Post("/reactions", a.postReaction)
	}

	// Reload router
	r.With(a.authMiddleware).Get("/reload", a.serveReloadRouter)
}

// Blog
func (a *goBlog) blogRouter(blog string, conf *configBlog) func(r chi.Router) {
	return func(r chi.Router) {

		// Set blog
		r.Use(middleware.WithValue(blogKey, blog))

		// Home
		r.Group(a.blogHomeRouter(conf))

		// Sections
		r.Group(a.blogSectionsRouter(conf))

		// Taxonomies
		r.Group(a.blogTaxonomiesRouter(conf))

		// Dates
		r.Group(a.blogDatesRouter(conf))

		// Photos
		r.Group(a.blogPhotosRouter(conf))

		// Search
		r.Group(a.blogSearchRouter(conf))

		// Random post
		r.Group(a.blogRandomRouter(conf))

		// On this day
		r.Group(a.blogOnThisDayRouter(conf))

		// Editor
		r.Route(conf.getRelativePath(editorPath), a.blogEditorRouter(conf))

		// Comments
		r.Group(a.blogCommentsRouter(conf))

		// Stats
		r.Group(a.blogStatsRouter(conf))

		// Blogroll
		r.Group(a.blogBlogrollRouter(conf))

		// Geo map
		r.Group(a.blogGeoMapRouter(conf))

		// Contact
		r.Group(a.blogContactRouter(conf))

		// Sitemap
		r.Group(a.blogSitemapRouter(conf))

		// Settings
		r.Route(conf.getRelativePath(settingsPath), a.blogSettingsRouter(conf))

	}
}

// Blog - Home
func (a *goBlog) blogHomeRouter(conf *configBlog) func(r chi.Router) {
	return func(r chi.Router) {
		if !conf.PostAsHome {
			r.Use(a.privateModeHandler)
			r.With(a.checkActivityStreamsRequest, a.cacheMiddleware).Get(conf.getRelativePath(""), a.serveHome)
			r.With(a.cacheMiddleware).Get(conf.getRelativePath("")+feedPath, a.serveHome)
			r.With(a.cacheMiddleware).Get(conf.getRelativePath(paginationPath), a.serveHome)
		}
	}
}

// Blog - Sections
func (a *goBlog) blogSectionsRouter(conf *configBlog) func(r chi.Router) {
	return func(r chi.Router) {
		r.Use(
			a.privateModeHandler,
			a.cacheMiddleware,
		)
		for _, section := range conf.Sections {
			r.Group(func(r chi.Router) {
				secPath := conf.getRelativePath(section.Name)
				r.Use(middleware.WithValue(indexConfigKey, &indexConfig{
					path:    secPath,
					section: section,
				}))
				registerIndexRoutes(r, secPath, a.serveIndex)
				r.Group(a.dateRoutes(conf, section.Name))
			})
		}
	}
}

// Blog - Taxonomies
func (a *goBlog) blogTaxonomiesRouter(conf *configBlog) func(r chi.Router) {
	return func(r chi.Router) {
		r.Use(
			a.privateModeHandler,
			a.cacheMiddleware,
		)
		for _, taxonomy := range conf.Taxonomies {
			if taxonomy.Name != "" {
				r.Group(func(r chi.Router) {
					r.Use(middleware.WithValue(taxonomyContextKey, taxonomy))
					taxBasePath := conf.getRelativePath(taxonomy.Name)
					r.Get(taxBasePath, a.serveTaxonomy)
					taxValPath := taxBasePath + "/{taxValue}"
					registerIndexRoutes(r, taxValPath, a.serveTaxonomyValue)
				})
			}
		}
	}
}

// Blog - Dates
func (a *goBlog) blogDatesRouter(conf *configBlog) func(r chi.Router) {
	return func(r chi.Router) {
		r.Use(
			a.privateModeHandler,
			a.cacheMiddleware,
		)

		r.Group(a.dateRoutes(conf, ""))
	}
}

func (a *goBlog) dateRoutes(conf *configBlog, pathPrefix string) func(r chi.Router) {
	return func(r chi.Router) {
		yearPath := conf.getRelativePath(pathPrefix + `/{year:(x|\d{4})}`)
		registerIndexRoutes(r, yearPath, a.serveDate)

		monthPath := yearPath + `/{month:(x|\d{2})}`
		registerIndexRoutes(r, monthPath, a.serveDate)

		dayPath := monthPath + `/{day:(\d{2})}`
		registerIndexRoutes(r, dayPath, a.serveDate)
	}
}

// Blog - Photos
func (a *goBlog) blogPhotosRouter(conf *configBlog) func(r chi.Router) {
	return func(r chi.Router) {
		if pc := conf.Photos; pc != nil && pc.Enabled {
			photoPath := conf.getRelativePath(cmp.Or(pc.Path, defaultPhotosPath))
			r.Use(
				a.privateModeHandler,
				a.cacheMiddleware,
				middleware.WithValue(indexConfigKey, &indexConfig{
					path:            photoPath,
					parameter:       a.cfg.Micropub.PhotoParam,
					title:           pc.Title,
					description:     pc.Description,
					summaryTemplate: photoSummary,
				}),
			)
			registerIndexRoutes(r, photoPath, a.serveIndex)
		}
	}
}

// Blog - Search
func (a *goBlog) blogSearchRouter(conf *configBlog) func(r chi.Router) {
	return func(r chi.Router) {
		if bsc := conf.Search; bsc != nil && bsc.Enabled {
			searchPath := conf.getRelativePath(cmp.Or(bsc.Path, defaultSearchPath))
			r.Route(searchPath, func(r chi.Router) {
				r.Group(func(r chi.Router) {
					r.Use(
						a.privateModeHandler,
						a.cacheMiddleware,
						middleware.WithValue(pathKey, searchPath),
					)
					r.Get("/", a.serveSearch)
					r.With(bodylimit.BodyLimit(10*bodylimit.KB)).Post("/", a.serveSearch)
					searchResultPath := "/" + searchPlaceholder
					registerIndexRoutes(r, searchResultPath, a.serveSearchResult)
				})
				r.With(
					// No private mode, to allow using OpenSearch in browser
					a.cacheMiddleware,
					middleware.WithValue(pathKey, searchPath),
				).Get("/opensearch.xml", a.serveOpenSearch)
			})
		}
	}
}

// Blog - Random
func (a *goBlog) blogRandomRouter(conf *configBlog) func(r chi.Router) {
	return func(r chi.Router) {
		if rp := conf.RandomPost; rp != nil && rp.Enabled {
			r.With(a.privateModeHandler).Get(conf.getRelativePath(cmp.Or(rp.Path, defaultRandomPath)), a.redirectToRandomPost)
		}
	}
}

// Blog - On this day
func (a *goBlog) blogOnThisDayRouter(conf *configBlog) func(r chi.Router) {
	return func(r chi.Router) {
		if otd := conf.OnThisDay; otd != nil && otd.Enabled {
			r.With(a.privateModeHandler).Get(conf.getRelativePath(cmp.Or(otd.Path, defaultOnThisDayPath)), a.redirectToOnThisDay)
		}
	}
}

// Blog - Editor
func (a *goBlog) blogEditorRouter(_ *configBlog) func(r chi.Router) {
	return func(r chi.Router) {
		r.Use(a.authMiddleware)
		r.Get("/", a.serveEditor)
		r.With(bodylimit.BodyLimit(50*bodylimit.MB)).Post("/", a.serveEditorPost)
		r.Get(editorFilesPath, a.serveEditorFiles)
		r.With(bodylimit.BodyLimit(100*bodylimit.KB)).Post(editorFileViewPath, a.serveEditorFilesView)
		r.With(bodylimit.BodyLimit(100*bodylimit.KB)).Post(editorFileUsesPath, a.serveEditorFilesUses)
		r.Get(editorFileUsesPath+editorFileUsesPathPlaceholder, a.serveEditorFilesUsesResults)
		r.Get(editorFileUsesPath+editorFileUsesPathPlaceholder+paginationPath, a.serveEditorFilesUsesResults)
		r.With(bodylimit.BodyLimit(100*bodylimit.KB)).Post(editorFileDeletePath, a.serveEditorFilesDelete)
		registerIndexRoutes(r, "/drafts", a.serveDrafts)
		registerIndexRoutes(r, "/private", a.servePrivate)
		registerIndexRoutes(r, "/unlisted", a.serveUnlisted)
		registerIndexRoutes(r, "/scheduled", a.serveScheduled)
		registerIndexRoutes(r, "/deleted", a.serveDeleted)
		r.HandleFunc("/ws", a.serveEditorWebsocket)
	}
}

// Blog - Comments
func (a *goBlog) blogCommentsRouter(conf *configBlog) func(r chi.Router) {
	return func(r chi.Router) {
		if a.commentsEnabled(conf) {
			commentsPath := conf.getRelativePath(commentPath)
			r.Route(commentsPath, func(r chi.Router) {
				r.Use(
					a.privateModeHandler,
					middleware.WithValue(pathKey, commentsPath),
				)
				r.With(a.cacheMiddleware, noIndexHeader).Get("/{id:[0-9]+}", a.serveComment)
				r.With(a.captchaMiddleware, bodylimit.BodyLimit(bodylimit.MB)).Post("/", a.createCommentFromRequest)
				r.Group(func(r chi.Router) {
					// Admin
					r.Use(a.authMiddleware)
					r.Get("/", a.commentsAdmin)
					r.Get(paginationPath, a.commentsAdmin)
					r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(commentDeleteSubPath, a.commentsAdminDelete)
					r.Get(commentEditSubPath, a.serveCommentsEditor)
					r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(commentEditSubPath, a.serveCommentsEditor)
				})
			})
		}
	}
}

// Blog - Stats
func (a *goBlog) blogStatsRouter(conf *configBlog) func(r chi.Router) {
	return func(r chi.Router) {
		if bsc := conf.BlogStats; bsc != nil && bsc.Enabled {
			statsPath := conf.getRelativePath(cmp.Or(bsc.Path, defaultBlogStatsPath))
			r.Use(a.privateModeHandler)
			r.With(a.cacheMiddleware).Get(statsPath, a.serveBlogStats)
		}
	}
}

// Blog - Blogroll
func (a *goBlog) blogBlogrollRouter(conf *configBlog) func(r chi.Router) {
	return func(r chi.Router) {
		if brEnabled, brPath := conf.getBlogrollPath(); brEnabled {
			r.Use(
				a.privateModeHandler,
				middleware.WithValue(cacheExpirationKey, a.defaultCacheExpiration()),
			)
			r.With(a.cacheMiddleware).Get(brPath, a.serveBlogroll)
			r.With(a.cacheMiddleware).Get(brPath+blogrollDownloadFile, a.serveBlogrollExport)
			r.With(a.authMiddleware).Post(brPath+blogrollRefreshSubpath, a.refreshBlogroll)
		}
	}
}

// Blog - Geo Map
func (a *goBlog) blogGeoMapRouter(conf *configBlog) func(r chi.Router) {
	return func(r chi.Router) {
		if mc := conf.Map; mc != nil && mc.Enabled {
			r.Use(a.privateModeHandler, a.cacheMiddleware)
			mapPath := conf.getRelativePath(cmp.Or(mc.Path, defaultGeoMapPath))
			r.Get(mapPath, a.serveGeoMap)
			r.Get(mapPath+geoMapTracksSubpath, a.serveGeoMapTracks)
			r.Get(mapPath+geoMapLocationsSubpath, a.serveGeoMapLocations)
		}
	}
}

// Blog - Contact
func (a *goBlog) blogContactRouter(conf *configBlog) func(r chi.Router) {
	return func(r chi.Router) {
		if cc := conf.Contact; cc != nil && cc.Enabled {
			contactPath := conf.getRelativePath(cmp.Or(cc.Path, defaultContactPath))
			r.Route(contactPath, func(r chi.Router) {
				r.Use(a.privateModeHandler, a.cacheMiddleware)
				r.Get("/", a.serveContactForm)
				r.With(a.captchaMiddleware, bodylimit.BodyLimit(bodylimit.MB)).Post("/", a.sendContactSubmission)
			})
		}
	}
}

// Blog - Sitemap
func (a *goBlog) blogSitemapRouter(conf *configBlog) func(r chi.Router) {
	return func(r chi.Router) {
		r.Use(a.privateModeHandler, cacheLoggedIn, a.cacheMiddleware)
		r.Get(conf.getRelativePath(sitemapBlogPath), a.serveSitemapBlog)
		r.Get(conf.getRelativePath(sitemapBlogFeaturesPath), a.serveSitemapBlogFeatures)
		r.Get(conf.getRelativePath(sitemapBlogArchivesPath), a.serveSitemapBlogArchives)
		r.Get(conf.getRelativePath(sitemapBlogPostsPath), a.serveSitemapBlogPosts)
	}
}

// Blog - Settings
func (a *goBlog) blogSettingsRouter(_ *configBlog) func(r chi.Router) {
	return func(r chi.Router) {
		r.Use(a.authMiddleware)
		r.Get("/", a.serveSettings)
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsDeleteSectionPath, a.settingsDeleteSection)
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsUpdateSectionPath, a.settingsUpdateSection)
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsUpdateDefaultSectionPath, a.settingsUpdateDefaultSection)
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsHideOldContentWarningPath, a.settingsHideOldContentWarning())
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsHideShareButtonPath, a.settingsHideShareButton())
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsHideTranslateButtonPath, a.settingsHideTranslateButton())
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsHideSpeakButtonPath, a.settingsHideSpeakButton())
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsAddReplyTitlePath, a.settingsAddReplyTitle())
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsAddReplyContextPath, a.settingsAddReplyContext())
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsAddLikeTitlePath, a.settingsAddLikeTitle())
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsAddLikeContextPath, a.settingsAddLikeContext())
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsUpdateReactionsEnabledPath, a.settingsUpdateReactionsEnabled())
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsUpdateReactionsPath, a.settingsUpdateReactions)
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsWebmentionDisableSendingPath, a.settingsWebmentionDisableSending())
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsWebmentionDisableReceivingPath, a.settingsWebmentionDisableReceiving())
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsWebmentionDisableInterGoblogPath, a.settingsWebmentionDisableInterGoblog())
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsWebmentionBlocklistAddPath, a.settingsWebmentionBlocklistAdd)
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsWebmentionBlocklistRemovePath, a.settingsWebmentionBlocklistRemove)
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsUpdateUserPath, a.settingsUpdateUser)
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsUpdateBlogPath, a.settingsUpdateBlog)
		r.With(bodylimit.BodyLimit(30*bodylimit.MB)).Post(settingsUpdateProfileImagePath, a.serveUpdateProfileImage)
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsDeleteProfileImagePath, a.serveDeleteProfileImage)
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsDeletePasskeyPath, a.settingsDeletePasskey)
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsRenamePasskeyPath, a.settingsRenamePasskey)
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsUpdatePasswordPath, a.settingsUpdatePassword)
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsDeletePasswordPath, a.settingsDeletePassword)
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsSetupTOTPPath, a.settingsSetupTOTP)
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsDeleteTOTPPath, a.settingsDeleteTOTP)
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsCreateAppPasswordPath, a.settingsCreateAppPassword)
		r.With(bodylimit.BodyLimit(bodylimit.MB)).Post(settingsDeleteAppPasswordPath, a.settingsDeleteAppPassword)
	}
}
