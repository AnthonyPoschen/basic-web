const loaded = new Set();

const scan = () => {
	const toLoad = new Set();
	const walk = (root) => {
		root.querySelectorAll(':not(:defined)').forEach(el => {
			const name = el.tagName.toLowerCase();
			if (name.includes('-') && !loaded.has(name)) toLoad.add(name);
		});
		root.querySelectorAll('*').forEach(el => el.shadowRoot && walk(el.shadowRoot));
	};
	walk(document.documentElement);

	toLoad.forEach(name => {
		loaded.add(name);
		fetch(`component/${name}.html`)
			.then(r => r.text())
			.then(html => {
				const div = document.createElement('div');
				div.innerHTML = html;
				const template = div.querySelector('template');
				if (template) document.head.appendChild(template);
				div.querySelectorAll('script').forEach(s => {
					const ns = document.createElement('script');
					if (s.type) ns.type = s.type;          // ← preserves type="module"
					if (s.src) {
						ns.src = s.src;
						ns.async = false;                    // preserve load order
					} else {
						ns.textContent = s.textContent;
					}
					document.head.appendChild(ns);
				});
				setTimeout(scan, 0);
			});
	});
};

document.addEventListener('DOMContentLoaded', scan);
document.addEventListener('htmx:afterSettle', scan);

if (window.location.hostname === 'localhost') {
	// Hot reloading
	new EventSource('/dev/reload').onmessage = () => { console.log("refresh recieved"); location.reload(); }
}
