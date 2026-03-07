const loaded = new Set();
const componentBasePath = '/component/';
const componentManifestPath = '/component-manifest.json';
let componentManifest = {};
let componentManifestPromise;
let isScanning = false;
let scanRequested = false;

const loadComponentManifest = () => {
	if (componentManifestPromise) return componentManifestPromise;

	componentManifestPromise = fetch(componentManifestPath)
		.then(response => {
			if (!response.ok) throw new Error(`failed to load component manifest: ${response.status}`);
			return response.json();
		})
		.then(manifest => {
			componentManifest = manifest && typeof manifest === 'object' ? manifest : {};
			return componentManifest;
		})
		.catch(error => {
			console.error(error);
			componentManifest = {};
			return componentManifest;
		});

	return componentManifestPromise;
};

const resolveComponentUrl = (name) => {
	const relativePath = componentManifest[name] || `${name}.html`;
	return new URL(relativePath, `${window.location.origin}${componentBasePath}`);
};

const collectUndefinedComponents = () => {
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

const appendComponentAssets = (html, componentUrl) => {
	const div = document.createElement('div');
	div.innerHTML = html;
	const template = div.querySelector('template');
	if (template) document.head.appendChild(template);
	div.querySelectorAll('script').forEach(s => {
		const ns = document.createElement('script');
		if (s.type) ns.type = s.type;
		if (s.src) {
			ns.src = new URL(s.getAttribute('src'), componentUrl).href;
			ns.async = false;
		} else {
			ns.textContent = s.textContent;
		}
		document.head.appendChild(ns);
	});
};

const loadComponent = async (name) => {
	loaded.add(name);
	const componentUrl = resolveComponentUrl(name);

	try {
		const response = await fetch(componentUrl);
		if (!response.ok) {
			throw new Error(`failed to load component ${name}: ${response.status}`);
		}

		appendComponentAssets(await response.text(), componentUrl);
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
		await loadComponentManifest();

		do {
			scanRequested = false;
			const names = collectUndefinedComponents();
			for (const name of names) {
				await loadComponent(name);
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

window.componentLoader = {
	loadManifest: loadComponentManifest,
	resolveUrl: resolveComponentUrl,
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
