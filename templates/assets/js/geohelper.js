(function () {
    let geoBtn = document.querySelector('#geobtn')
    function geo() {
        let status = document.querySelector('#geostatus')
        status.classList.add('hide')
        status.value = ''

        function success(position) {
            let latitude = position.coords.latitude
            let longitude = position.coords.longitude
            status.value = `geo:${latitude},${longitude}`
            status.classList.remove('hide')
        }

        function error() {
            alert(geoBtn.dataset.failed)
        }

        if (navigator.geolocation) {
            navigator.geolocation.getCurrentPosition(success, error)
        } else {
            alert(geoBtn.dataset.notsupported)
        }
    }
    geoBtn.addEventListener('click', geo)
})()