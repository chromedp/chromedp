function getClientRect() {
  const e = this.getBoundingClientRect(),
    t = this.ownerDocument.documentElement.getBoundingClientRect();
  return {
    x: e.left - t.left,
    y: e.top - t.top,
    width: e.width,
    height: e.height,
  };
}
