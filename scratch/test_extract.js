const fs = require('fs');
const jsdom = require('jsdom');
const { JSDOM } = jsdom;
const html = fs.readFileSync('scratch/test.html', 'utf8');
const dom = new JSDOM(html);
const window = dom.window;
const document = window.document;

function queryData(target, hint) {
    const ALL_TAGS = 'p, div, span, h1, h2, h3, h4, h5, h6, li, td, th, label, strong, b, i, em';
    const stopWords = { 'a': 1, 'an': 1, 'the': 1, 'of': 1, 'in': 1, 'is': 1, 'to': 1, 'for': 1 };
    
    target = target.toLowerCase();
    hint = hint.toLowerCase();

    var hintWords = hint.split(/\s+/).filter(w => !stopWords[w] && w.length >= 2);
    var targetWords = target.split(/\s+/).filter(w => !stopWords[w] && w.length >= 2);

    if (targetWords.length > 0) {
        var bestRow = null;
        var bestScore = 0;
        var rows = Array.from(document.querySelectorAll('tr'));
        for (var r = 0; r < rows.length; r++) {
            var rowText = Array.from(rows[r].querySelectorAll('td')).map(c => c.innerHTML.toLowerCase()).join(' ');
            var tds = Array.from(rows[r].querySelectorAll('td'));
            if (tds.length === 0) continue;

            var table = rows[r].closest('table');
            var ths = table ? Array.from(table.querySelectorAll('th')) : [];
            var colWords = targetWords.filter(w => ths.some(th => th.innerHTML.toLowerCase().includes(w)));
            var rowWords = targetWords.filter(w => colWords.indexOf(w) < 0);

            if (rowWords.length > 0 && Math.round(rowWords.filter(w => rowText.includes(w)).length / rowWords.length) >= 1) {
                var colIdx = -1;
                if (colWords.length > 0) {
                    for (var ti = 0; ti < ths.length; ti++) {
                        if (colWords.some(w => ths[ti].innerHTML.toLowerCase().includes(w))) { colIdx = ti; break; }
                    }
                }
                if (colIdx >= 0 && colIdx < tds.length) return tds[colIdx].innerHTML.trim();
                return "FAIL COLIDX";
            }
        }
    }
    return "FAIL STRATEGY 1";
}

console.log(queryData('CPU of Chrome', ''));
