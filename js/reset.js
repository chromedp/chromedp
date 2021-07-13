function reset() {
    if (this.nodeName === 'FORM') {
        HTMLFormElement.prototype.reset.call(this);
        return true;
    } else if (this.form !== null) {
        HTMLFormElement.prototype.reset.call(this.form);
        return true;
    }
    return false;
}
