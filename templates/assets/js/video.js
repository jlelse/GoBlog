(function () {
    function loadVideo() {
        // Get video div element
        let videoDivEl = document.getElementById('video')

        // External Video URL
        let videoUrl = videoDivEl.dataset.url

        // Create video element
        let videoEl = document.createElement('video')
        videoEl.controls = true
        videoEl.classList.add('fw')

        // Load video
        if (Hls.isSupported()) {
            let hls = new Hls()
            hls.loadSource(videoUrl)
            hls.attachMedia(videoEl)
        } else if (videoEl.canPlayType('application/vnd.apple.mpegurl')) {
            videoEl.src = videoUrl
        }

        // Add video element
        videoDivEl.appendChild(videoEl)
    }

    // JS
    let script = document.createElement('script')
    script.src = '/-/hlsjs/hls.js?v=1.4.14'
    script.integrity = 'sha256-WFJL6071ADbK6R3m+8ujK7v2QFiKX/aD2JDm4KgZTIo='
    script.onload = loadVideo
    document.head.appendChild(script)
})()