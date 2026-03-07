const loaded = new Set();
const elementBasePath = '/elements/';
const elementManifestPath = '/framework/element-manifest.json';
let elementManifest = {};
let elementManifestPromise;
let isScanning = false;
let scanRequested = false;

const loadElementManifest = () => {
	if (elementManifestPromise) return elementManifestPromise;

	elementManifestPromise = fetch(elementManifestPath)
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

const resolveElementUrl = (name) => {
	const relativePath = elementManifest[name] || `${name}.html`;
	return new URL(relativePath, `${window.location.origin}${elementBasePath}`);
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

const appendelementAssets = (html, elementUrl) => {
	const div = document.createElement('div');
	div.innerHTML = html;
	const template = div.querySelector('template');
	if (template) document.head.appendChild(template);
	div.querySelectorAll('script').forEach(s => {
		const ns = document.createElement('script');
		if (s.type) ns.type = s.type;
		if (s.src) {
			ns.src = new URL(s.getAttribute('src'), elementUrl).href;
			ns.async = false;
		} else {
			ns.textContent = s.textContent;
		}
		document.head.appendChild(ns);
	});
};

const loadElement = async (name) => {
	loaded.add(name);
	const elementUrl = resolveElementUrl(name);

	try {
		const response = await fetch(elementUrl);
		if (!response.ok) {
			throw new Error(`failed to load element ${name}: ${response.status}`);
		}

		appendelementAssets(await response.text(), elementUrl);
		scanRequested = true;
	} catch (error) {
		loaded.delete(name);
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
