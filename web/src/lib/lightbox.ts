// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. See the LICENSE file.

// Svelte action: click-to-enlarge for <img> inside the node (the in-app guide
// bodies), opening a full-size overlay. Footer logos are excluded.
export function lightbox(node: HTMLElement) {
	function open(src: string, alt: string) {
		const overlay = document.createElement('div');
		overlay.className = 'doc-lightbox';
		overlay.setAttribute('role', 'dialog');
		overlay.setAttribute('aria-label', alt || 'Screenshot');

		const img = document.createElement('img');
		img.src = src;
		img.alt = alt;
		overlay.appendChild(img);

		const hint = document.createElement('div');
		hint.className = 'doc-lightbox-hint';
		hint.textContent = 'Click anywhere or press Esc to close';
		overlay.appendChild(hint);

		function close() {
			overlay.remove();
			document.removeEventListener('keydown', onKey);
		}
		function onKey(e: KeyboardEvent) {
			if (e.key === 'Escape') close();
		}
		overlay.addEventListener('click', close);
		document.addEventListener('keydown', onKey);
		document.body.appendChild(overlay);
	}

	function onClick(e: MouseEvent) {
		const t = e.target as HTMLElement;
		if (t.tagName !== 'IMG' || t.closest('footer')) return;
		const im = t as HTMLImageElement;
		if (!im.src) return;
		e.preventDefault();
		open(im.src, im.alt);
	}

	node.addEventListener('click', onClick);
	return {
		destroy() {
			node.removeEventListener('click', onClick);
		}
	};
}
