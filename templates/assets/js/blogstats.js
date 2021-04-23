(function () {
    Array.from(document.getElementsByClassName('statsyear')).forEach(element => {
        element.addEventListener('click', function () {
            Array.from(document.getElementsByClassName('statsmonth')).forEach(c => {
                if (element.dataset.year == c.dataset.year) {
                    c.classList.contains('hide') ? c.classList.remove('hide') : c.classList.add('hide')
                }
            })
        })
    })
})()