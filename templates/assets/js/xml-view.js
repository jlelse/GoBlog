(function () {
    const XHTML = 'http://www.w3.org/1999/xhtml';

    function create(name, attributes = {}, ...children) {
        const element = document.createElementNS(XHTML, name);
        for (const [key, value] of Object.entries(attributes)) {
            element.setAttribute(key, value);
        }
        element.append(...children);
        return element;
    }

    function directChild(element, name) {
        for (const child of element.children) {
            if (child.localName === name) {
                return child;
            }
        }
        return null;
    }

    function childElements(element, name) {
        return [...element.children].filter(child => child.localName === name);
    }

    function text(element) {
        return element ? element.textContent.trim() : '';
    }

    function attr(element, name) {
        return element.getAttribute(name) || '';
    }

    function renderPage(title, main) {
        const head = [
            create('meta', {name: 'viewport', content: 'width=device-width, initial-scale=1'}),
            create('title', {}, title)
        ];

        const script = document.getElementsByTagNameNS(XHTML, 'script')[0];
        const css = script && script.getAttribute('data-css');
        if (css) {
            const attributes = {rel: 'stylesheet', href: css};
            const integrity = script.getAttribute('data-css-integrity');
            if (integrity) {
                attributes.integrity = integrity;
            }
            head.push(create('link', attributes));
        }

        const html = create('html', {},
            create('head', {}, ...head),
            create('body', {}, main)
        );

        document.documentElement.replaceWith(html);
    }

    function renderSitemap(root) {
        const isIndex = root.localName === 'sitemapindex';
        const entryName = isIndex ? 'sitemap' : 'url';
        const entries = [];

        for (const child of childElements(root, entryName)) {
            const loc = text(directChild(child, 'loc'));
            if (!loc) {
                continue;
            }
            entries.push({loc: loc, lastMod: text(directChild(child, 'lastmod'))});
        }

        const title = isIndex ? 'Sitemap index' : 'Sitemap';
        const articles = entries.map(entry => {
            const link = create('a', {href: entry.loc});
            link.textContent = entry.loc;
            const article = create('article', {class: 'border-bottom'},
                create('h2', {}, link)
            );
            if (entry.lastMod) {
                const time = create('time', {datetime: entry.lastMod});
                time.textContent = entry.lastMod;
                article.append(create('p', {}, time));
            }
            return article;
        });

        renderPage(title, create('main', {},
            create('h1', {}, title),
            create('hr'),
            ...articles
        ));
    }

    function isFeed(outline) {
        return Boolean(attr(outline, 'xmlUrl') || attr(outline, 'htmlUrl'));
    }

    function outlineTitle(outline) {
        return attr(outline, 'title') || attr(outline, 'text');
    }

    function collectFeeds(outline) {
        const feeds = [];
        for (const child of childElements(outline, 'outline')) {
            if (isFeed(child)) {
                feeds.push(child);
            } else {
                feeds.push(...collectFeeds(child));
            }
        }
        return feeds;
    }

    function feedItem(outline) {
        const htmlUrl = attr(outline, 'htmlUrl');
        const xmlUrl = attr(outline, 'xmlUrl');
        const description = attr(outline, 'description');
        const title = outlineTitle(outline) || htmlUrl || xmlUrl;
        const item = create('li');
        item.append(create('a', {
            href: htmlUrl || xmlUrl,
            target: '_blank',
            rel: 'noopener noreferrer'
        }, title));
        if (xmlUrl) {
            item.append(' (', create('a', {
                href: xmlUrl,
                target: '_blank',
                rel: 'noopener noreferrer'
            }, 'Feed'), ')');
        }
        if (description) {
            item.append(' — ', create('i', {}, description));
        }
        return item;
    }

    function renderOPML(root) {
        const head = directChild(root, 'head');
        const body = directChild(root, 'body');
        const title = text(head && directChild(head, 'title'));

        const script = document.getElementsByTagNameNS(XHTML, 'script')[0];
        const website = script && script.getAttribute('data-website');

        const main = create('main', {},
            create('h1', {}, title || 'Blogroll')
        );
        if (website) {
            main.append(create('p', {}, create('a', {href: website}, website)));
        }
        main.append(create('hr'));

        let looseFeeds = null;
        for (const outline of body ? childElements(body, 'outline') : []) {
            if (isFeed(outline)) {
                if (!looseFeeds) {
                    looseFeeds = create('ul');
                    main.append(looseFeeds);
                }
                looseFeeds.append(feedItem(outline));
                continue;
            }
            looseFeeds = null;
            const feeds = collectFeeds(outline);
            if (!feeds.length) {
                continue;
            }
            main.append(create('h2', {}, outlineTitle(outline) + ' (' + feeds.length + ')'));
            const list = create('ul');
            for (const feed of feeds) {
                list.append(feedItem(feed));
            }
            main.append(list);
        }

        renderPage(title || 'Blogroll', main);
    }

    function render() {
        const root = document.documentElement;
        if (!root) {
            return;
        }
        switch (root.localName) {
            case 'urlset':
            case 'sitemapindex':
                renderSitemap(root);
                break;
            case 'opml':
                renderOPML(root);
                break;
        }
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', render, {once: true});
    } else {
        render();
    }
})();
