(() => {
    const setupWS = (element) => {
        if (!element.dataset.ws) return;
        const wsParams = new URLSearchParams(element.dataset.ws.split('?')[1]);
        const preview = wsParams.get('preview') === '1';
        const sync = wsParams.get('sync') === '1';

        const previewContainer = preview ? document.getElementById(element.dataset.preview) : null;
        if (preview && !previewContainer) return;

        let ws = null;

        const openWS = () => {
            try {
                ws = new WebSocket(`${window.location.protocol === "https:" ? "wss://" : "ws://"}${window.location.host}${element.dataset.ws}`);

                ws.onopen = () => {
                    console.log("Editor WebSocket opened");
                };

                ws.onclose = () => {
                    console.log("Editor WebSocket closed, reopening in 1 second");
                    if (preview) {
                        previewContainer.classList.add('hide');
                        previewContainer.classList.remove('preview');
                        previewContainer.innerHTML = '';
                    }
                    ws = null;
                    setTimeout(openWS, 1000);
                };

                ws.onmessage = (evt) => {
                    const msg = evt.data;
                    if (sync && msg.startsWith("sync:")) {
                        element.value = msg.slice(5);
                    } else if (preview && msg.startsWith("preview:")) {
                        previewContainer.classList.add('preview');
                        previewContainer.classList.remove('hide');
                        previewContainer.innerHTML = msg.slice(8);
                    } else if (msg.startsWith("formatted:")) {
                        const parts = msg.slice(10).split(":");
                        const cursor = parseInt(parts.pop(), 10);
                        element.value = parts.join(":");
                        element.selectionStart = element.selectionEnd = cursor;
                        element.focus();
                        element.dispatchEvent(new Event("input"));
                    } else if (msg === "triggerpreview") {
                        if (ws && ws.readyState === WebSocket.OPEN) {
                            ws.send(element.value);
                        }
                    }
                };

                ws.onerror = (evt) => {
                    console.log("Editor WebSocket error:", evt.data);
                };
            } catch (error) {
                console.error("Failed to create editor WebSocket:", error);
                setTimeout(openWS, 1000);
            }
        };

        openWS();

        let debounceTimeout;
        element.addEventListener('input', () => {
            clearTimeout(debounceTimeout);
            debounceTimeout = setTimeout(() => {
                if (ws && ws.readyState === WebSocket.OPEN) {
                    ws.send(element.value);
                }
            }, 500);
        });

        element.form.addEventListener('submit', () => {
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.send('');
            }
        });

        const toolbar = element.previousElementSibling;
        if (toolbar && toolbar.classList.contains('editor-toolbar')) {
            toolbar.addEventListener('click', (e) => {
                const btn = e.target.closest('button[data-action]');
                if (!btn || !ws || ws.readyState !== WebSocket.OPEN) return;
                const action = btn.dataset.action;
                const start = element.selectionStart;
                const end = element.selectionEnd;
                ws.send(`format:${action}:${start}:${end}:${element.value}`);
            });
        }
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
        document.querySelector('#templatebtn').addEventListener('confirmed', () => {
            const area = document.querySelector('#editor-create');
            area.value = area.dataset.template;
        });
    };

    document.querySelectorAll('#editor-create, #editor-update').forEach(setupWS);
    setupGeoButton();
    setupTemplateButton();
})();