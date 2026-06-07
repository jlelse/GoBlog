(() => {
    const PREFIX_SYNC = "sync:";
    const PREFIX_PREVIEW = "preview:";
    const PREFIX_FORMATTED = "formatted:";

    const execUndoableReplace = (element, newContent, cursorPos) => {
        element.focus();
        element.select();
        document.execCommand('insertText', false, newContent);
        element.selectionStart = element.selectionEnd = cursorPos;
    };

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
                    if (sync && msg.startsWith(PREFIX_SYNC)) {
                        const newContent = msg.slice(PREFIX_SYNC.length);
                        if (element.value !== newContent) {
                            execUndoableReplace(element, newContent, element.selectionStart);
                        }
                    } else if (preview && msg.startsWith(PREFIX_PREVIEW)) {
                        previewContainer.classList.add('preview');
                        previewContainer.classList.remove('hide');
                        previewContainer.innerHTML = msg.slice(PREFIX_PREVIEW.length);
                    } else if (msg.startsWith(PREFIX_FORMATTED)) {
                        const parts = msg.slice(PREFIX_FORMATTED.length).split(":");
                        const cursor = parseInt(parts.pop(), 10);
                        const newContent = parts.join(":");
                        execUndoableReplace(element, newContent, cursor);
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
                if (!btn) return;
                const action = btn.dataset.action;
                if (action === 'undo' || action === 'redo') {
                    element.focus();
                    document.execCommand(action);
                } else if (ws && ws.readyState === WebSocket.OPEN) {
                    const start = element.selectionStart;
                    const end = element.selectionEnd;
                    ws.send(`format:${action}:${start}:${end}:${element.value}`);
                }
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