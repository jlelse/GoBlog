"use strict";

let sb = document.getElementById('speakBtn')
let s = window.speechSynthesis

function gv() {
    return s ? s.getVoices().filter(voice => voice.lang.startsWith(document.querySelector('html').lang))[0] : false
}

function is() {
    if (s) {
        sb.classList.remove('hide')
        sb.onclick = sp
        sb.textContent = sb.dataset.speak
    }
}

function sp() {
    sb.onclick = ssp
    sb.textContent = sb.dataset.stopspeak
    let ut = new SpeechSynthesisUtterance(
        ((document.querySelector('article .p-name')) ? document.querySelector('article .p-name').innerText + "\n\n" : '') + document.querySelector('article .e-content').innerText
    )
    ut.voice = gv()
    ut.onerror = ssp
    ut.onend = ssp
    s.speak(ut)
}

function ssp() {
    s.cancel()
    sb.onclick = sp
    sb.textContent = sb.dataset.speak
}

window.onbeforeunload = ssp
is()