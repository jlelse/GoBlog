(function () {
    let speakButton = query('#speakBtn')
    let ttsAudio = query('#tts-audio')

    let init = false

    speakButton.textContent = speakButton.dataset.speak
    speakButton.classList.remove('hide')
    speakButton.addEventListener('click', function () {
        if (!init) {
            init = true
            query('#tts').classList.remove('hide')
            ttsAudio.play()
        } else {
            togglePlay()
        }
    })

    let isPlaying = false

    function togglePlay() {
        isPlaying ? ttsAudio.pause() : ttsAudio.play()
    }
    ttsAudio.onplaying = function() {
        isPlaying = true
        speakButton.textContent = speakButton.dataset.stopspeak
    }
    ttsAudio.onpause = function() {
        isPlaying = false
        speakButton.textContent = speakButton.dataset.speak
    }

    function query(selector) {
        return document.querySelector(selector)
    }
})()