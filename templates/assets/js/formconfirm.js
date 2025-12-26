(function () {
    const selector = 'form input.confirm, form button.confirm'

    function setupConfirm(element) {
        if (!element.form) return
        
        let showed = false
        const isInput = element.tagName.toLowerCase() === 'input'

        const setBusy = () => {
            if (isInput) element.value = '…'
            else element.textContent = '…'
        }

        const setConfirm = () => {
            const msg = element.dataset.confirmmessage
            if (isInput) element.value = msg
            else element.textContent = msg
            showed = true
        }

        element.form.addEventListener('submit', event => {
            if (event.submitter === element && !showed) {
                event.preventDefault()
                setBusy()
                setTimeout(setConfirm, 1000)
                return false
            }
        })
    }

    Array.from(document.querySelectorAll(selector)).forEach(setupConfirm)
})()