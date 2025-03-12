(function () {
    const loadingEl = document.getElementById('loading');
    const tableUrl = loadingEl.dataset.table;

    const tableReq = new XMLHttpRequest();
    tableReq.open('GET', tableUrl);
    tableReq.onload = function () {
        if (tableReq.status === 200) {
            const parser = new DOMParser();
            const doc = parser.parseFromString(tableReq.responseText, 'text/html');
            const table = doc.querySelector('table');

            if (table) {
                loadingEl.outerHTML = table.outerHTML;
                document.querySelectorAll('.statsyear').forEach(element => {
                    element.addEventListener('click', function () {
                        document.querySelectorAll('.statsmonth').forEach(c => {
                            if (element.dataset.year === c.dataset.year) {
                                c.classList.toggle('hide');
                            }
                        });
                    });
                });
            }
        }
    };
    tableReq.send();
})();