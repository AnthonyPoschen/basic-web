const loaded = new Set();
const loadsByURL = new Map();
const elementBasePath = '/elements/';
const elementManifestPath = '/framework/element-manifest.json';
const webVersion = document.documentElement.dataset.webVersion || '';
let elementManifest = {};
let elementManifestPromise;
let isScanning = false;
let scanRequested = false;

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

const collectUndefinedelements = () => {
	const toLoad = new Set();
	const walk = (root) => {
		root.querySelectorAll(':not(:defined)').forEach(el => {
			const name = el.tagName.toLowerCase();
			if (name.includes('-') && !loaded.has(name)) toLoad.add(name);
		});
		root.querySelectorAll('*').forEach(el => el.shadowRoot && walk(el.shadowRoot));
	};

	walk(document.documentElement);
	return toLoad;
};

const appendElementAssets = async (html, elementUrl, elementName) => {
	const div = document.createElement('div');
	div.innerHTML = html;
	const template = div.querySelector('template');
	if (template) document.head.appendChild(template);
	const moduleDefinitions = [];
	div.querySelectorAll('script').forEach(s => {
		const ns = document.createElement('script');
		if (s.type) ns.type = s.type;
		if (ns.type === 'module') {
			// Inline module scripts do not reliably emit a load event when injected.
			// The custom-element definition is the completion signal the loader needs.
			moduleDefinitions.push(customElements.whenDefined(elementName));
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

const loadElement = async (name) => {
	const elementUrl = resolveElementUrl(name);
	const key = elementUrl.href;
	let load = loadsByURL.get(key);
	if (!load) {
		load = (async () => {
			const response = await fetch(elementUrl);
			if (!response.ok) {
				throw new Error(`failed to load element ${name}: ${response.status}`);
			}

			await appendElementAssets(await response.text(), elementUrl, name);
		})();
		loadsByURL.set(key, load);
	}

	try {
		await load;
		loaded.add(name);
		scanRequested = true;
	} catch (error) {
		loadsByURL.delete(key);
		console.error(error);
	}
};

const runScanLoop = async () => {
	if (isScanning) return;
	isScanning = true;

	try {
		await loadElementManifest();

		do {
			scanRequested = false;
			const names = collectUndefinedelements();
			for (const name of names) {
				await loadElement(name);
			}
		} while (scanRequested);
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
