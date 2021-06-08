function setAttribute(n, v) {
    this[n] = v;
    if (n === 'value') {
        this.dispatchEvent(new Event('input', {bubbles: true}));
        this.dispatchEvent(new Event('change', {bubbles: true}));
    }
    return this[n];
}
