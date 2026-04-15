([target, hint]) => {
    var ALL_TAGS = 'div, span, p, h1, h2, h3, h4, h5, h6, li, dd, dt, '
        + 'strong, b, i, em, label, a, button, td, th, article, section';
    var hintWords = hint ? hint.split(/\s+/).filter(function(w) { return w.length > 1; }) : [];
    var wordMatch = function(text, word) {
        if (word.length >= 5) return text.includes(word);
        var re = new RegExp('\\b' + word.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '\\b', 'i');
        return re.test(text);
    };

    // Gather all <th> text once for column/row word splitting
    var allTables = Array.from(document.querySelectorAll('table'));
    var allThTexts = [];
    allTables.forEach(function(t) {
        Array.from(t.querySelectorAll('th')).forEach(function(th) {
            allThTexts.push(th.innerText.toLowerCase().trim());
        });
    });

    // Split target words into column-words (found in headers) and row-words (the rest)
    var classifyWords = function(words) {
        var colWords = [];
        var rowWords = [];
        for (var i = 0; i < words.length; i++) {
            var w = words[i];
            var isCol = allThTexts.some(function(th) { return wordMatch(th, w); });
            if (isCol) { colWords.push(w); } else { rowWords.push(w); }
        }
        return { col: colWords, row: rowWords };
    };

    // Strategy 1: Search table rows for word matches
    if (target) {
        var targetWords = target.split(/\s+/).filter(function(w) { return w.length > 2; });
        var classified = classifyWords(targetWords);
        // Use row-words for row matching; fall back to all targetWords if no row-words
        var searchWords = classified.row.length > 0 ? classified.row : targetWords;

        var rows = Array.from(document.querySelectorAll('tr, [role="row"]'));
        var bestRow = null;
        var maxMatches = 0;
        var bestLen = Infinity;
        for (var ri = 0; ri < rows.length; ri++) {
            var rText = rows[ri].innerText.toLowerCase();
            // Skip header rows when we have row-specific search words
            var isHeader = rows[ri].querySelectorAll('th').length > 0 && rows[ri].querySelectorAll('td').length === 0;
            if (isHeader && classified.row.length > 0) continue;
            var matches = 0;
            if (searchWords.length === 0) {
                if (wordMatch(rText, target)) matches = 1;
            } else {
                for (var wi = 0; wi < searchWords.length; wi++) {
                    if (wordMatch(rText, searchWords[wi])) matches++;
                }
            }
            if (matches > maxMatches) {
                maxMatches = matches;
                bestLen = rText.length;
                bestRow = rows[ri];
            } else if (matches === maxMatches && maxMatches > 0) {
                if (rText.length < bestLen) { bestLen = rText.length; bestRow = rows[ri]; }
            }
        }
        if (bestRow) {
            var cells = Array.from(bestRow.querySelectorAll('td'));
            if (cells.length === 0) cells = Array.from(bestRow.querySelectorAll('[role="cell"]'));
            if (cells.length > 1) {
                // Try column header matching using column-words
                var colSearchWords = classified.col.length > 0 ? classified.col : hintWords;
                if (colSearchWords.length > 0) {
                    var table = bestRow.closest('table');
                    if (table) {
                        var ths = Array.from(table.querySelectorAll('th'));
                        for (var hw = 0; hw < colSearchWords.length; hw++) {
                            for (var ti = 0; ti < ths.length; ti++) {
                                if (wordMatch(ths[ti].innerText.toLowerCase(), colSearchWords[hw])) {
                                    if (ti < cells.length) return cells[ti].innerText.trim();
                                }
                            }
                        }
                    }
                }
                // Try hint-based cell matching
                if (hint) {
                    var hintIdx = -1;
                    for (var hi = 0; hi < cells.length; hi++) {
                        if (cells[hi].innerText.toLowerCase().includes(hint)) { hintIdx = hi; break; }
                    }
                    if (hintIdx >= 0 && hintIdx + 1 < cells.length) return cells[hintIdx + 1].innerText.trim();
                    for (var hi2 = 0; hi2 < cells.length; hi2++) {
                        if (cells[hi2].innerText.toLowerCase().includes(hint)) return cells[hi2].innerText.trim();
                    }
                }
                // Try numeric/currency cell
                for (var ci = 0; ci < cells.length; ci++) {
                    var ct = cells[ci].innerText.trim();
                    if (/[$\u20ac\u00a3%\u20b4]/.test(ct) || ct.includes('Rs.') || !isNaN(parseFloat(ct))) return ct;
                }
                return cells[cells.length - 1].innerText.trim();
            }
            if (cells.length === 1) return cells[0].innerText.trim();
            return bestRow.innerText.trim();
        }
    }
    // Strategy 2: hint-only with column resolution
    if (hintWords.length > 0) {
        var rows2 = Array.from(document.querySelectorAll('tr'));
        for (var r2i = 0; r2i < rows2.length; r2i++) {
            var row = rows2[r2i];
            var cells2 = Array.from(row.querySelectorAll('td'));
            if (cells2.length < 2) continue;
            var rowText = cells2.map(function(c) { return c.innerText.toLowerCase(); }).join(' ');
            var table2 = row.closest('table');
            var ths2 = table2 ? Array.from(table2.querySelectorAll('th')) : [];
            var colWords = hintWords.filter(function(w) { return ths2.some(function(th) { return th.innerText.toLowerCase().includes(w); }); });
            var rowWords = hintWords.filter(function(w) { return colWords.indexOf(w) < 0; });
            if (rowWords.length > 0 && rowWords.every(function(w) { return rowText.includes(w); })) {
                if (colWords.length > 0) {
                    var colIdx2 = -1;
                    for (var ti2 = 0; ti2 < ths2.length; ti2++) {
                        if (colWords.some(function(w) { return ths2[ti2].innerText.toLowerCase().includes(w); })) { colIdx2 = ti2; break; }
                    }
                    if (colIdx2 >= 0 && colIdx2 < cells2.length) return cells2[colIdx2].innerText.trim();
                }
                for (var ci2 = 0; ci2 < cells2.length; ci2++) {
                    var ct2 = cells2[ci2].innerText.trim();
                    if (/[$\u20ac\u00a3%\u20b4]/.test(ct2) || !isNaN(parseFloat(ct2))) return ct2;
                }
                return cells2[cells2.length - 1].innerText.trim();
            }
        }
    }
    // Strategy 3: broad text search
    if (target) {
        var allEls = Array.from(document.querySelectorAll(ALL_TAGS));
        for (var ei = 0; ei < allEls.length; ei++) {
            var txt = (allEls[ei].innerText || '').trim().toLowerCase();
            if (!txt || txt.length > 200) continue;
            if (txt === target || txt.includes(target)) {
                var kids = allEls[ei].querySelectorAll(ALL_TAGS);
                var leafKids = Array.from(kids).filter(function(k) {
                    return (k.innerText || '').trim().toLowerCase().includes(target);
                });
                if (leafKids.length > 0) return leafKids[leafKids.length - 1].innerText.trim();
                return allEls[ei].innerText.trim();
            }
        }
    }
    return '';
}
