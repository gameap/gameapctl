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
    conn.send(JSON.stringify({
        "topic": topic,
        "code": "payload",
        "value": message
    }))
}

conn.onclose = function (event) {
    log.value += "\nConnection closed.\n"
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