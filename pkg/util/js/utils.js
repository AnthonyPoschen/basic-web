const globalSheet = new CSSStyleSheet();
function syncStyles() {
	let css = '';
	for (const sheet of document.styleSheets) {
		try { css += [...sheet.cssRules].map(r => r.cssText).join('\n'); } catch { }
	}
	globalSheet.replaceSync(`@layer global {\n${css}\n}`);
}
syncStyles();
document.addEventListener('DOMContentLoaded', syncStyles);
addEventListener('load', syncStyles);
window.globalSheet = globalSheet;

class ShadowHTMLElement extends HTMLElement {
	constructor(templateID) {
		super();
		syncStyles();
		this.attachShadow({ mode: 'open' });
		this.shadowRoot.adoptedStyleSheets = [window.globalSheet];
		this.template = document.getElementById(templateID);
		this.shadowRoot.appendChild(this.template.content.cloneNode(true));
	}
}
