package main

import "github.com/writeas/go-nodeinfo"

var (
	nodeInfoConfig  *nodeinfo.Config
	nodeInfoService *nodeinfo.Service
)

func initNodeInfo() {
	nodeInfoConfig = &nodeinfo.Config{
		BaseURL: appConfig.Server.PublicAddress,
		InfoURL: "/nodeinfo",
		Metadata: nodeinfo.Metadata{
			NodeName:        appConfig.Blogs[appConfig.DefaultBlog].Title,
			NodeDescription: appConfig.Blogs[appConfig.DefaultBlog].Description,
		},
		Protocols: []nodeinfo.NodeProtocol{
			nodeinfo.ProtocolActivityPub,
			"micropub",
			"webmention",
		},
		Services: nodeinfo.Services{
			Inbound: []nodeinfo.NodeService{},
			Outbound: []nodeinfo.NodeService{
				nodeinfo.ServiceAtom,
				nodeinfo.ServiceRSS,
				"jsonfeed",
				"activitystreams2.0",
				"telegram",
			},
		},
		Software: nodeinfo.SoftwareInfo{
			Name: appUserAgent,
		},
	}
	nodeInfoService = nodeinfo.NewService(*nodeInfoConfig, &nodeInfoResolver{})
}

type nodeInfoResolver struct{}

func (r *nodeInfoResolver) IsOpenRegistration() (bool, error) {
	return false, nil
}

func (r *nodeInfoResolver) Usage() (nodeinfo.Usage, error) {
	postCount, _ := countPosts(&postsRequestConfig{})
	u := nodeinfo.Usage{
		Users: nodeinfo.UsageUsers{
			Total: len(appConfig.Blogs),
		},
		LocalPosts: postCount,
	}
	return u, nil
}
