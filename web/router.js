const normalizePath = (pathname) => {
	if (!pathname || pathname === '/') return '/';
	const normalized = pathname.endsWith('/') && pathname !== '/' ? pathname.slice(0, -1) : pathname;
	return normalized || '/';
};

const parseQuery = (search) => {
	const params = new URLSearchParams(search);
	const query = {};
	const keys = [];
	for (const [key, value] of params.entries()) {
		if (!(key in query)) query[key] = value;
		keys.push(key);
	}
	return {query, keys};
};

const routeDefinitions = [];
const routeSubscribers = new Set();

const matchRoute = (pathname) => {
	const normalizedPath = normalizePath(pathname);
	const pathSegments = normalizedPath === '/' ? [] : normalizedPath.slice(1).split('/').map(decodeURIComponent);

	for (const route of routeDefinitions) {
		if (route.segments.length !== pathSegments.length) continue;

		const params = {};
		let isMatch = true;
		for (let index = 0; index < route.segments.length; index++) {
			const routeSegment = route.segments[index];
			const pathSegment = pathSegments[index];
			if (routeSegment.type === 'param') {
				params[routeSegment.name] = pathSegment;
				continue;
			}
			if (routeSegment.value !== pathSegment) {
				isMatch = false;
				break;
			}
		}

		if (isMatch) {
			return {definition: route, params};
		}
	}

	return null;
};

const buildRouteState = () => {
	const {query, keys} = parseQuery(window.location.search);
	const matched = matchRoute(window.location.pathname);
	return {
		path: normalizePath(window.location.pathname),
		search: window.location.search,
		hash: window.location.hash,
		query,
		queryKeys: keys,
		params: matched?.params ?? {},
		pattern: matched?.definition.pattern ?? null,
		component: matched?.definition.component ?? null,
		meta: matched?.definition.meta ?? {},
	};
};

const notifyRouteSubscribers = () => {
	const route = buildRouteState();
	window.appRouter.current = route;
	routeSubscribers.forEach(listener => listener(route));
	return route;
};

const registerRoute = (pattern, component, meta = {}) => {
	const normalizedPattern = normalizePath(pattern);
	const segments = normalizedPattern === '/'
		? []
		: normalizedPattern.slice(1).split('/').map(segment => (
			segment.startsWith(':')
				? {type: 'param', name: segment.slice(1)}
				: {type: 'static', value: segment}
		));

	routeDefinitions.push({pattern: normalizedPattern, component, meta, segments});
	return window.appRouter;
};

const navigate = (target, options = {}) => {
	const nextUrl = new URL(target, window.location.origin);
	const nextPath = `${normalizePath(nextUrl.pathname)}${nextUrl.search}${nextUrl.hash}`;
	const currentPath = `${normalizePath(window.location.pathname)}${window.location.search}${window.location.hash}`;

	if (nextPath === currentPath) {
		return notifyRouteSubscribers();
	}

	if (!options.replace) {
		window.history.pushState({}, '', nextPath);
	} else {
		window.history.replaceState({}, '', nextPath);
	}

	return notifyRouteSubscribers();
};

window.appRouter = {
	current: null,
	register: registerRoute,
	navigate,
	subscribe(listener) {
		routeSubscribers.add(listener);
		if (this.current) listener(this.current);
		return () => routeSubscribers.delete(listener);
	},
	start() {
		return notifyRouteSubscribers();
	},
};

class RouteView extends HTMLElement {
	connectedCallback() {
		this.unsubscribe = window.appRouter.subscribe(route => this.render(route));
	}

	disconnectedCallback() {
		this.unsubscribe?.();
	}

	render(route) {
		const componentName = route.component || this.getAttribute('not-found');
		this.replaceChildren();

		if (!componentName) return;

		const page = document.createElement(componentName);
		page.route = route;
		page.setAttribute('data-route-path', route.path);
		if (route.pattern) page.setAttribute('data-route-pattern', route.pattern);
		Object.entries(route.params).forEach(([key, value]) => {
			page.setAttribute(`route-param-${key}`, value);
		});
		Object.entries(route.query).forEach(([key, value]) => {
			page.setAttribute(`route-query-${key}`, value);
		});
		this.appendChild(page);
		window.componentLoader?.scheduleScan?.();
	}
}

customElements.define('route-view', RouteView);

document.addEventListener('click', event => {
	if (event.defaultPrevented) return;
	if (event.button !== 0) return;
	if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;

	const link = event.target.closest('a[href]');
	if (!link) return;
	if (link.target && link.target !== '_self') return;
	if (link.hasAttribute('download')) return;

	const url = new URL(link.href, window.location.href);
	if (url.origin !== window.location.origin) return;
	if (!url.pathname.startsWith('/')) return;

	event.preventDefault();
	navigate(`${url.pathname}${url.search}${url.hash}`);
});

window.addEventListener('popstate', () => {
	notifyRouteSubscribers();
});

document.addEventListener('DOMContentLoaded', () => {
	window.appRouter.start();
});