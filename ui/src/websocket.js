const conn = new WebSocket("ws://localhost:17080/ws")

let subscribers= new Map()

function subscribe(topic, callback) {
    const id = Math.floor(Math.random() * Number.MAX_SAFE_INTEGER)
    subscribers.set(id, {topic: topic, callback: callback})
    return id
}

function unsubscribe(id) {
    subscribers.delete(id)
}

function send(topic, message) {
    waitForSocketConnection(conn, function(){
        conn.send(JSON.stringify({
            "topic": topic,
            "code": "payload",
            "value": message
        }))
    });
}

function waitForSocketConnection(socket, callback){
    setTimeout(
        function () {
            if (socket.readyState === WebSocket.OPEN) {
                if (callback != null){
                    callback();
                }
            } else {
                waitForSocketConnection(socket, callback);
            }

        }, 5);
}

conn.onclose = function (event) {
    console.log("Connection closed.")
};
conn.onmessage = function (event) {
    const msg = JSON.parse(event.data)
    for (const [key, value] of subscribers) {
        if (value.topic === msg.topic) {
            value.callback(key, msg.code, msg.value)
        }
    }
};

function unarySend(topic, message, callback) {
    subscribe(topic, (id, code, value) => {
        unsubscribe(id)
        callback(code, value)
    })
    send(topic, message)
}

export {subscribe, unsubscribe, send, unarySend}