(function () {
    let loadingEl = document.getElementById('loading')
    let tableUrl = loadingEl.dataset.table

    let tableReq = new XMLHttpRequest()
    tableReq.open('GET', tableUrl)
    tableReq.onload = function() {
        if (tableReq.status == 200) {
            loadingEl.outerHTML = tableReq.responseText
            Array.from(document.getElementsByClassName('statsyear')).forEach(element => {
                element.addEventListener('click', function () {
                    Array.from(document.getElementsByClassName('statsmonth')).forEach(c => {
                        if (element.dataset.year == c.dataset.year) {
                            c.classList.contains('hide') ? c.classList.remove('hide') : c.classList.add('hide')
                        }
                    })
                })
            })
        }
    }
    tableReq.send()
})()