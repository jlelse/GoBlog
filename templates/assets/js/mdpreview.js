(function () {
    Array.from(document.querySelectorAll('.mdpreview')).forEach(element => {
        // Get preview container
        let previewContainer = document.getElementById(element.dataset.preview)
        if (!previewContainer) {
            return
        }
        // Get websocket path
        let wsUrl = element.dataset.previewws
        if (!wsUrl) {
            return
        }
        // Create and open websocket
        let ws = new WebSocket(((window.location.protocol === "https:") ? "wss://" : "ws://") + window.location.host + wsUrl)
        ws.onopen = function () {
            console.log("Preview-Websocket opened")
            previewContainer.classList.add('preview')
            previewContainer.classList.remove('hide')
            if (ws) {
                ws.send(element.value)
            }
        }
        ws.onclose = function () {
            console.log("Preview-Websocket closed")
            previewContainer.classList.add('hide')
            previewContainer.classList.remove('preview')
            previewContainer.innerHTML = ''
            ws = null
        }
        ws.onmessage = function (evt) {
            // Set preview HTML
            previewContainer.innerHTML = evt.data
        }
        ws.onerror = function (evt) {
            console.log("Preview-Websocket error: " + evt.data)
        }
        // Add listener
        let timeout = null
        element.addEventListener('input', function () {
            clearTimeout(timeout)
            timeout = setTimeout(function () {
                if (ws) {
                    ws.send(element.value)
                }
            }, 500)
        })
    })
})()