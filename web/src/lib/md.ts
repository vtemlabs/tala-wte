// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

// Minimal Markdown renderer for in-app docs (authored in-repo, never user input);
// all text is HTML-escaped before inline formatting.

const escape = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

function inline(s: string): string {
	return escape(s)
		.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, '<img alt="$1" src="$2" loading="lazy" />')
		.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2">$1</a>')
		.replace(/`([^`]+)`/g, '<code>$1</code>')
		.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
}

const blockStart = /^(#{1,4}\s|>\s?|\s*[-*]\s+|\s*\d+\.\s+|```|---+\s*$|\s*\|)/;

export function mdToHtml(md: string): string {
	const lines = md.replace(/\r/g, '').split('\n');
	let html = '';
	let i = 0;

	while (i < lines.length) {
		const line = lines[i];

		if (line.trim().startsWith('```')) {
			i++;
			let code = '';
			while (i < lines.length && !lines[i].trim().startsWith('```')) {
				code += lines[i] + '\n';
				i++;
			}
			i++; // closing fence
			html += `<pre><code>${escape(code.replace(/\n$/, ''))}</code></pre>`;
			continue;
		}

		const h = line.match(/^(#{1,4})\s+(.*)$/);
		if (h) {
			const lvl = h[1].length;
			html += `<h${lvl}>${inline(h[2])}</h${lvl}>`;
			i++;
			continue;
		}

		if (/^---+\s*$/.test(line)) {
			html += '<hr />';
			i++;
			continue;
		}

		if (/^>\s?/.test(line)) {
			let q = '';
			while (i < lines.length && /^>\s?/.test(lines[i])) {
				q += inline(lines[i].replace(/^>\s?/, '')) + ' ';
				i++;
			}
			html += `<blockquote>${q.trim()}</blockquote>`;
			continue;
		}

		if (/^\s*[-*]\s+/.test(line)) {
			html += '<ul>';
			while (i < lines.length && /^\s*[-*]\s+/.test(lines[i])) {
				html += `<li>${inline(lines[i].replace(/^\s*[-*]\s+/, ''))}</li>`;
				i++;
			}
			html += '</ul>';
			continue;
		}

		if (/^\s*\d+\.\s+/.test(line)) {
			html += '<ol>';
			while (i < lines.length && /^\s*\d+\.\s+/.test(lines[i])) {
				html += `<li>${inline(lines[i].replace(/^\s*\d+\.\s+/, ''))}</li>`;
				i++;
			}
			html += '</ol>';
			continue;
		}

		// Tables: a header row, a |---| separator, then body rows.
		if (
			line.trim().startsWith('|') &&
			i + 1 < lines.length &&
			/^\s*\|?[\s:|-]*-[\s:|-]*\|?\s*$/.test(lines[i + 1])
		) {
			const cells = (row: string) =>
				row
					.trim()
					.replace(/^\||\|$/g, '')
					.split('|')
					.map((c) => c.trim());
			const headers = cells(line);
			i += 2; // header + separator
			let body = '';
			while (i < lines.length && lines[i].trim().startsWith('|')) {
				body += '<tr>' + cells(lines[i]).map((c) => `<td>${inline(c)}</td>`).join('') + '</tr>';
				i++;
			}
			html +=
				'<table><thead><tr>' +
				headers.map((c) => `<th>${inline(c)}</th>`).join('') +
				'</tr></thead><tbody>' +
				body +
				'</tbody></table>';
			continue;
		}

		if (line.trim() === '') {
			i++;
			continue;
		}

		let para = '';
		while (i < lines.length && lines[i].trim() !== '' && !blockStart.test(lines[i])) {
			para += (para ? ' ' : '') + lines[i];
			i++;
		}
		html += `<p>${inline(para)}</p>`;
	}

	return html;
}
