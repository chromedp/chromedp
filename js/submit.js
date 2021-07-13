function submit() {
    if (this.nodeName === 'FORM') {
        HTMLFormElement.prototype.submit.call(this);
        return true;
    } else if (this.form !== null) {
        HTMLFormElement.prototype.submit.call(this.form);
        return true;
    }
    return false;
}
