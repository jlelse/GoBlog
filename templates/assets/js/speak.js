(function () {
    window.onbeforeunload = stopSpeak

    let speakButton = query('#speakBtn')
    let speech = window.speechSynthesis

    if (speech) {
        speakButton.onclick = startSpeak
        speakButton.textContent = speakButton.dataset.speak
        speakButton.classList.remove('hide')
    }

    function query(selector) {
        return document.querySelector(selector)
    }

    function getVoice() {
        return speech ? speech.getVoices().filter(voice => voice.lang.startsWith(query('html').lang))[0] : false
    }

    function startSpeak() {
        speakButton.onclick = stopSpeak
        speakButton.textContent = speakButton.dataset.stopspeak
        let ut = new SpeechSynthesisUtterance(
            ((query('article .p-name')) ? query('article .p-name').innerText + "\n\n" : '') + query('article .e-content').innerText
        )
        ut.voice = getVoice()
        ut.onerror = stopSpeak
        ut.onend = stopSpeak
        speech.speak(ut)
    }

    function stopSpeak() {
        speech.cancel()
        speakButton.onclick = startSpeak
        speakButton.textContent = speakButton.dataset.speak
    }
})()