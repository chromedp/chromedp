function exposeFunc(name, binding) {
    var reply = binding + '_reply';
    var send = window[binding];
    if (Object.prototype.hasOwnProperty.call(window, reply) || typeof send !== 'function') {
        return;
    }
    var pending = new Map();
    var last = 0;
    Object.defineProperty(window, reply, {
        value: function (id, ok, value) {
            var call = pending.get(id);
            if (!call) {
                return;
            }
            pending.delete(id);
            if (ok) {
                call.resolve(value);
            } else {
                call.reject(new Error(value));
            }
        },
    });
    window[name] = function () {
        var args = Array.prototype.slice.call(arguments);
        return new Promise(function (resolve, reject) {
            var id = ++last;
            pending.set(id, {resolve: resolve, reject: reject});
            try {
                send(JSON.stringify({id: id, args: args}));
            } catch (e) {
                pending.delete(id);
                reject(e);
            }
        });
    };
}
