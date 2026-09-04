const globalSheet = new CSSStyleSheet();
let globalCSS = '';
function syncStyles() {
	let css = '';
	for (const sheet of document.styleSheets) {
		try { css += [...sheet.cssRules].map(r => r.cssText).join('\n'); } catch { }
	}
	if (css === globalCSS) return;
	globalCSS = css;
	globalSheet.replaceSync(`@layer global {\n${css}\n}`);
}
syncStyles();
document.addEventListener('DOMContentLoaded', syncStyles);
addEventListener('load', syncStyles);
window.globalSheet = globalSheet;

class ShadowHTMLElement extends HTMLElement {
	constructor(templateID) {
		super();
		const root = this.shadowRoot ?? this.attachShadow({ mode: 'open' });
		root.adoptedStyleSheets = [window.globalSheet];
		this.template = document.getElementById(templateID);
		if (!root.hasChildNodes() && this.template) {
			root.appendChild(this.template.content.cloneNode(true));
		}
	}
}
