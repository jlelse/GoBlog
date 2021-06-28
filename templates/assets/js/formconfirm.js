(function () {
    Array.from(document.querySelectorAll('form input.confirm')).forEach(element => {
        let showed = false
        element.form.addEventListener('submit', event => {
            if (event.submitter == element && !showed) {
                event.preventDefault()
                element.value = '…'
                setTimeout(() => {
                    element.value = element.dataset.confirmmessage
                    showed = true
                }, 1000)
                return false
            }
        })
    })
})()