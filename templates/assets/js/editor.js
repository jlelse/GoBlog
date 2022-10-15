(function () {
    // Preview
    function openPreviewWS(element) {
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
            console.log("Preview-Websocket closed, try to reopen in 1 second")
            previewContainer.classList.add('hide')
            previewContainer.classList.remove('preview')
            previewContainer.innerHTML = ''
            ws = null
            setTimeout(function () { openPreviewWS(element) }, 1000);
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
    }
    Array.from(document.querySelectorAll('#editor-create, #editor-update')).forEach(element => openPreviewWS(element))

    // Sync state
    function openSyncStateWS(element, initial) {
        // Get websocket path
        let wsUrl = element.dataset.syncws
        if (!wsUrl) {
            return
        }
        // Create and open websocket
        let ws = new WebSocket(((window.location.protocol === "https:") ? "wss://" : "ws://") + window.location.host + wsUrl + '?initial=' + initial)
        ws.onopen = function () {
            console.log("Sync-Websocket opened")
        }
        ws.onclose = function () {
            console.log("Sync-Websocket closed, try to reopen in 1 second")
            ws = null
            setTimeout(function () { openSyncStateWS(element, "0") }, 1000);
        }
        ws.onmessage = function (evt) {
            element.value = evt.data
        }
        ws.onerror = function (evt) {
            console.log("Sync-Websocket error: " + evt.data)
        }
        // Add listener
        let timeout = null
        element.addEventListener('input', function () {
            clearTimeout(timeout)
            timeout = setTimeout(function () {
                if (ws) {
                    ws.send(element.value)
                }
            }, 100)
        })
        // Clear on submit
        element.form.addEventListener('submit', function () {
            if (ws) {
                ws.send('')
            }
        })
    }
    Array.from(document.querySelectorAll('#editor-create')).forEach(element => openSyncStateWS(element, "1"))

    // Geo button
    let geoBtn = document.querySelector('#geobtn')
    geoBtn.addEventListener('click', function () {
        let status = document.querySelector('#geostatus')
        status.classList.add('hide')
        status.value = ''

        function success(position) {
            let latitude = position.coords.latitude
            let longitude = position.coords.longitude
            status.value = `geo:${latitude},${longitude}`
            status.classList.remove('hide')
        }

        function error() {
            alert(geoBtn.dataset.failed)
        }

        if (navigator.geolocation) {
            navigator.geolocation.getCurrentPosition(success, error)
        } else {
            alert(geoBtn.dataset.notsupported)
        }
    })

    // Template button
    document.querySelector('#templatebtn').addEventListener('click', function () {
        let area = document.querySelector('#editor-create')
        area.value = area.dataset.template;
    })
})()