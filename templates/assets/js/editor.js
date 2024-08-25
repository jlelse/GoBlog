(() => {
    const createWebSocket = (url) => {
        return new WebSocket(`${window.location.protocol === "https:" ? "wss://" : "ws://"}${window.location.host}${url}`);
    };

    const handleWebSocketEvents = (ws, callbacks) => {
        if (ws) {
            ws.onopen = callbacks.onOpen;
            ws.onclose = callbacks.onClose;
            ws.onmessage = callbacks.onMessage;
            ws.onerror = callbacks.onError;
        }
    };

    const debounce = (func, delay) => {
        let timeout;
        return (...args) => {
            clearTimeout(timeout);
            timeout = setTimeout(() => func(...args), delay);
        };
    };

    const setupPreviewWS = (element) => {
        const previewContainer = document.getElementById(element.dataset.preview);
        if (!previewContainer || !element.dataset.previewws) return;

        let ws = null;

        const openPreviewWS = () => {
            try {
                ws = createWebSocket(element.dataset.previewws);

                handleWebSocketEvents(ws, {
                    onOpen: () => {
                        console.log("Preview-Websocket opened");
                        previewContainer.classList.add('preview');
                        previewContainer.classList.remove('hide');
                        ws.send(element.value);
                    },
                    onClose: () => {
                        console.log("Preview-Websocket closed, reopening in 1 second");
                        previewContainer.classList.add('hide');
                        previewContainer.classList.remove('preview');
                        previewContainer.innerHTML = '';
                        ws = null;
                        setTimeout(openPreviewWS, 1000);
                    },
                    onMessage: (evt) => {
                        previewContainer.innerHTML = evt.data;
                    },
                    onError: (evt) => {
                        console.log("Preview-Websocket error:", evt.data);
                    }
                });
            } catch (error) {
                console.error("Failed to create Preview WebSocket:", error);
                setTimeout(openPreviewWS, 1000);
            }
        };

        openPreviewWS();

        element.addEventListener('input', debounce(() => {
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.send(element.value);
            }
        }, 500));
    };

    const setupSyncStateWS = (element) => {
        if (!element.dataset.syncws) return;

        let ws = null;

        const openSyncStateWS = () => {
            try {
                ws = createWebSocket(element.dataset.syncws);

                handleWebSocketEvents(ws, {
                    onOpen: () => console.log("Sync-Websocket opened"),
                    onClose: () => {
                        console.log("Sync-Websocket closed, reopening in 1 second");
                        ws = null;
                        setTimeout(openSyncStateWS, 1000);
                    },
                    onMessage: (evt) => {
                        element.value = evt.data;
                    },
                    onError: (evt) => {
                        console.log("Sync-Websocket error:", evt.data);
                    }
                });
            } catch (error) {
                console.error("Failed to create Sync WebSocket:", error);
                setTimeout(openSyncStateWS, 1000);
            }
        };

        openSyncStateWS();

        element.addEventListener('input', debounce(() => {
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.send(element.value);
            }
        }, 100));

        element.form.addEventListener('submit', () => {
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.send('');
            }
        });
    };

    const setupGeoButton = () => {
        const geoBtn = document.querySelector('#geobtn');
        const status = document.querySelector('#geostatus');

        geoBtn.addEventListener('click', () => {
            status.classList.add('hide');
            status.value = '';

            const success = (position) => {
                const { latitude, longitude } = position.coords;
                status.value = `geo:${latitude},${longitude}`;
                status.classList.remove('hide');
            };

            const error = () => {
                alert(geoBtn.dataset.failed);
            };

            if (navigator.geolocation) {
                navigator.geolocation.getCurrentPosition(success, error);
            } else {
                alert(geoBtn.dataset.notsupported);
            }
        });
    };

    const setupTemplateButton = () => {
        document.querySelector('#templatebtn').addEventListener('click', () => {
            const area = document.querySelector('#editor-create');
            area.value = area.dataset.template;
        });
    };

    document.querySelectorAll('#editor-create, #editor-update').forEach(setupPreviewWS);
    document.querySelectorAll('#editor-create').forEach(setupSyncStateWS);
    setupGeoButton();
    setupTemplateButton();
})();