"use strict";

function getVoice() {
    if (window.speechSynthesis) {
        return window.speechSynthesis.getVoices().filter(voice => voice.lang.startsWith(document.querySelector('html').lang))[0];
    }
    return false;
}

function initSpeak() {
    if (window.speechSynthesis) {
        let speakBtn = document.querySelector('#speakBtn');
        speakBtn.style.display = '';
        speakBtn.onclick = function() { speak() };
        speakBtn.textContent = speakText;
    }
}

function speak() {
    console.log("Start speaking")
    let speakBtn = document.querySelector('#speakBtn');
    speakBtn.onclick = function() { stopSpeak() };
    speakBtn.textContent = stopSpeakText;
    let textContent =
        ((document.querySelector('article .p-name')) ? document.querySelector('article .p-name').innerText + "\n\n" : "")
        + document.querySelector('article .e-content').innerText;
    let utterThis = new SpeechSynthesisUtterance(textContent);
    utterThis.voice = getVoice();
    utterThis.onerror = stopSpeak;
    utterThis.onend = stopSpeak;
    window.speechSynthesis.speak(utterThis);
}

function stopSpeak() {
    console.log("Stop speaking")
    window.speechSynthesis.cancel();
    let speakBtn = document.querySelector('#speakBtn');
    speakBtn.onclick = function() { speak() };
    speakBtn.textContent = speakText;
}

window.onbeforeunload = function () {
    stopSpeak();
}
initSpeak();