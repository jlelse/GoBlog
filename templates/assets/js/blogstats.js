(function () {
    document.querySelectorAll('.statsyear').forEach(element => {
        element.addEventListener('click', function () {
            document.querySelectorAll('.statsmonth').forEach(c => {
                if (element.dataset.year === c.dataset.year) {
                    c.classList.toggle('hide');
                }
            });
        });
    });
})();