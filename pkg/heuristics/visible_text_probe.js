(() => {
    let t = (document.body.innerText || "") + " ";
    document.querySelectorAll('*').forEach(el => {
        const st = window.getComputedStyle(el);
        const isHidden = st.display === 'none' || st.visibility === 'hidden' || st.opacity === '0';
        if (isHidden) return;
        if (el.title)       t += el.title + " ";
        if (el.value && typeof el.value === 'string') t += el.value + " ";
        if (el.placeholder) t += el.placeholder + " ";
        const ariaLabel = el.getAttribute && el.getAttribute('aria-label');
        if (ariaLabel) t += ariaLabel + " ";
        const ariaValText = el.getAttribute && el.getAttribute('aria-valuetext');
        if (ariaValText) t += ariaValText + " ";
        if (el.shadowRoot)
            t += Array.from(el.shadowRoot.querySelectorAll('*'))
                      .map(e => e.innerText || e.value || '').join(' ');
    });
    return t.toLowerCase();
})()
