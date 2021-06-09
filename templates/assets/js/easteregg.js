(function () {
    let input = '', key = 'ArrowUpArrowUpArrowDownArrowDownArrowLeftArrowRightArrowLeftArrowRightba'
    document.addEventListener('keyup', function (e) {
        input += e.key
        if (input == key) {
            input = ''
            document.documentElement.classList.add('egg-anim')
            document.documentElement.classList.toggle('turn-around')
        }
        if (input != '' && !key.startsWith(input)) {
            input = ''
        }
    })
})()