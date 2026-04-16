(xpath) => {
    const result = document.evaluate(
        xpath, document, null,
        XPathResult.FIRST_ORDERED_NODE_TYPE, null
    );
    const el = result.singleNodeValue;
    if (!el) return null;
    const rect = el.getBoundingClientRect();
    const cs = window.getComputedStyle(el);
    const vis = !(cs.display === 'none' || cs.visibility === 'hidden' || parseFloat(cs.opacity) < 0.01)
                && rect.width > 0 && rect.height > 0;
    return {
        found:      true,
        tag:        el.tagName.toLowerCase(),
        is_visible: vis,
        is_disabled: el.disabled || el.getAttribute('aria-disabled') === 'true',
        rect: {
            top: rect.top, left: rect.left,
            bottom: rect.bottom, right: rect.right,
            width: rect.width, height: rect.height
        }
    };
}
