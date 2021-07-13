function text() {
    if (this.offsetWidth || this.offsetHeight || this.getClientRects().length) {
        return this.innerText;
    }
    return '';
}
