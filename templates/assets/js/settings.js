(function () {

    // Make checkboxes with class '.autosubmit' submit form automatically
    Array.from(document.querySelectorAll("form input[type='checkbox'].autosubmit")).forEach(element => {
        element.addEventListener('change', event => {
            event.currentTarget.closest('form').submit()
        })
    })

})()