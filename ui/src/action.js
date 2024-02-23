function runAction(dialogTitle, dialogContent, topic, message) {
  subscribers.forEach(subscriber => {
    subscriber(dialogTitle, dialogContent, topic, message)
  })
}

function runActionWithoutDialog(topic, message) {
  subscribers.forEach(subscriber => {
    subscriber("", "", topic, message)
  })
}

let subscribers = []

function subscribeAction(callback) {
  subscribers.push(callback)
}

export { runAction, runActionWithoutDialog, subscribeAction }