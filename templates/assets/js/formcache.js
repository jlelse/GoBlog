(function () {
    const fc = 'formcache'
    Array.from(document.querySelectorAll('form .' + fc)).forEach(element => {
        let elementName = fc + '-' + location.pathname + '#' + element.id
        // Load from cache
        let cached = localStorage.getItem(elementName)
        if (cached != null) {
            element.value = cached
        }
        // Auto save to cache
        element.addEventListener('input', function () {
            localStorage.setItem(elementName, element.value)
        })
        // Clear on submit
        element.form.addEventListener('submit', function () {
            localStorage.removeItem(elementName)
        })
    })
})()