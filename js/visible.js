function visible() {
    return Boolean(this.offsetWidth || this.offsetHeight || this.getClientRects().length);
}
