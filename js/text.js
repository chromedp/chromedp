function text() {
    // A text node has no layout of its own and no innerText, so use the
    // element that holds it.
    const el = this.nodeType === 1 ? this : this.parentElement;
    if (el && (el.offsetWidth || el.offsetHeight || el.getClientRects().length)) {
        return this.nodeType === 1 ? this.innerText : this.nodeValue;
    }
    return '';
}
