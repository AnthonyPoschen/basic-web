const loaded = new Set();
const loadsByURL = new Map();
const elementBasePath = '/elements/';
const elementManifestPath = '/framework/element-manifest.json';
const webVersion = document.documentElement.dataset.webVersion || '';
let elementManifest = {};
let elementManifestPromise;
let isScanning = false;
let scanRequested = false;

const loadingAttribute = 'data-basic-web-loading';
const loadingStyle = document.createElement('style');
loadingStyle.textContent = `[${loadingAttribute}] { visibility: hidden !important; }`;
document.head.appendChild(loadingStyle);

const loadElementManifest = () => {
	if (elementManifestPromise) return elementManifestPromise;

	elementManifestPromise = fetch(versionedUrl(elementManifestPath))
		.then(response => {
			if (!response.ok) throw new Error(`failed to load element manifest: ${response.status}`);
			return response.json();
		})
		.then(manifest => {
			elementManifest = manifest && typeof manifest === 'object' ? manifest : {};
			return elementManifest;
		})
		.catch(error => {
			console.error(error);
			elementManifest = {};
			return elementManifest;
		});

	return elementManifestPromise;
};

const versionedUrl = (path, base = window.location.origin) => {
	const url = new URL(path, base);
	if (webVersion) url.searchParams.set('v', webVersion);
	return url;
};

void loadElementManifest();

const resolveElementUrl = (name) => {
	const relativePath = elementManifest[name] || `${name}.html`;
	return versionedUrl(relativePath, `${window.location.origin}${elementBasePath}`);
};

const elementName = (element) => element.tagName.toLowerCase();

const isLoadableElement = (element) => {
	const name = elementName(element);
	return name.includes('-') && !loaded.has(name) && !customElements.get(name);
};

const collectUndefinedElements = () => {
	const toLoad = new Set();
	const walk = (root) => {
		root.querySelectorAll(':not(:defined)').forEach(el => {
			if (isLoadableElement(el)) toLoad.add(elementName(el));
		});
		root.querySelectorAll('*').forEach(el => el.shadowRoot && walk(el.shadowRoot));
	};

	walk(document.documentElement);
	return toLoad;
};

const collectElementDependencies = (html) => {
	const container = document.createElement('div');
	container.innerHTML = html;
	const dependencies = new Set();

	container.querySelectorAll('template').forEach(template => {
		template.content.querySelectorAll('*').forEach(element => {
			const name = elementName(element);
			if (name.includes('-')) dependencies.add(name);
		});
	});

	return dependencies;
};

const appendElementTemplates = (html) => {
	const div = document.createElement('div');
	div.innerHTML = html;
	div.querySelectorAll('template').forEach(template => document.head.appendChild(template));
};

const appendElementScripts = async (html, elementUrl, elementNames) => {
	const div = document.createElement('div');
	div.innerHTML = html;
	const moduleDefinitions = [];
	div.querySelectorAll('script').forEach(s => {
		const ns = document.createElement('script');
		if (s.type) ns.type = s.type;
		if (ns.type === 'module') {
			// Inline module scripts do not reliably emit a load event when injected.
			// The custom-element definition is the completion signal the loader needs.
			elementNames.forEach(name => moduleDefinitions.push(customElements.whenDefined(name)));
		} else if (s.src) {
			moduleDefinitions.push(new Promise((resolve, reject) => {
				ns.addEventListener('load', resolve, { once: true });
				ns.addEventListener('error', () => reject(new Error(`failed to execute element script: ${elementUrl}`)), { once: true });
			}));
		}
		if (s.src) {
			ns.src = new URL(s.getAttribute('src'), elementUrl).href;
			ns.async = false;
		} else {
			ns.textContent = s.textContent;
		}
		document.head.appendChild(ns);
	});
	await Promise.all(moduleDefinitions);
};

const loadElementSource = async (name) => {
	const elementUrl = resolveElementUrl(name);
	const key = elementUrl.href;
	let source = loadsByURL.get(key);
	if (!source) {
		source = (async () => {
			const response = await fetch(elementUrl);
			if (!response.ok) {
				throw new Error(`failed to load element ${name}: ${response.status}`);
			}

			const html = await response.text();
			return {
				html,
				elementUrl,
				names: new Set(),
				dependencies: collectElementDependencies(html),
			};
		})();
		loadsByURL.set(key, source);
	}

	try {
		const resolved = await source;
		resolved.names.add(name);
		return resolved;
	} catch (error) {
		loadsByURL.delete(key);
		throw error;
	}
};

const installElementSource = (source) => {
	if (!source.install) {
		source.install = (async () => {
			appendElementTemplates(source.html);
			await appendElementScripts(source.html, source.elementUrl, [...source.names]);
		})();
	}
	return source.install;
};

const loadElementTree = async (initialNames) => {
	await loadElementManifest();

	const namesToVisit = new Set(initialNames);
	const visitedNames = new Set();
	const sources = new Map();

	while (namesToVisit.size > 0) {
		const names = [...namesToVisit];
		namesToVisit.clear();
		const newNames = names.filter(name => {
			if (visitedNames.has(name)) return false;
			visitedNames.add(name);
			return name.includes('-') && !loaded.has(name) && !customElements.get(name);
		});
		if (newNames.length === 0) continue;

		const batch = await Promise.all(newNames.map(loadElementSource));
		batch.forEach(source => {
			sources.set(source.elementUrl.href, source);
			source.dependencies.forEach(dependency => {
				if (!visitedNames.has(dependency)) namesToVisit.add(dependency);
			});
		});
	}

	await Promise.all([...sources.values()].map(installElementSource));
	visitedNames.forEach(name => {
		if (customElements.get(name)) loaded.add(name);
	});
	scanRequested = true;
};

const hydrate = async (root) => {
	const name = root && elementName(root);
	if (!name || !name.includes('-') || customElements.get(name)) return;

	root.setAttribute(loadingAttribute, '');
	try {
		await loadElementTree([name]);
	} catch (error) {
		console.error(error);
	} finally {
		root.removeAttribute(loadingAttribute);
		root.dispatchEvent(new CustomEvent('basic-web:ready', { bubbles: true, composed: true }));
	}
};

const runScanLoop = async () => {
	if (isScanning) return;
	isScanning = true;

	try {
		await loadElementManifest();

	do {
			scanRequested = false;
			const names = collectUndefinedElements();
			if (names.size > 0) await loadElementTree(names);
		} while (scanRequested);
	} catch (error) {
		console.error(error);
	} finally {
		isScanning = false;
		if (scanRequested) {
			void runScanLoop();
		}
	}
};

const requestScan = () => {
	scanRequested = true;
	void runScanLoop();
};

window.elementLoader = {
	loadManifest: loadElementManifest,
	resolveUrl: resolveElementUrl,
	hydrate,
	scan: requestScan,
	scheduleScan: requestScan,
};

document.addEventListener('DOMContentLoaded', requestScan);
document.addEventListener('htmx:afterSettle', requestScan);

if (window.location.hostname === 'localhost') {
	// Hot reloading
	new EventSource('/dev/reload').onmessage = () => { console.log("refresh recieved"); location.reload(); }
}
// Auto-scan shadow DOM + light DOM changes
const origAttach = Element.prototype.attachShadow;
Element.prototype.attachShadow = function(...a) {
	const sr = origAttach.apply(this, a);
	requestScan();
	return sr;
};
new MutationObserver(() => requestScan())
	.observe(document.documentElement, { childList: true, subtree: true });
