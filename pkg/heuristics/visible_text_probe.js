() => {
    let t = (document.body ? document.body.innerText : '') + ' ';
    document.querySelectorAll('[aria-label],[placeholder],[title],[value]').forEach(el => {
        const cs = window.getComputedStyle(el);
        if (cs.display === 'none' || cs.visibility === 'hidden') return;
        t += ' ' + (el.getAttribute('aria-label') || '');
        t += ' ' + (el.getAttribute('placeholder') || '');
        t += ' ' + (el.getAttribute('title') || '');
        if (el.value) t += ' ' + el.value;
    });
    return { url: window.location.href, text: t.toLowerCase() };
}
