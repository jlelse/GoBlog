(function () {
    function createSnowflake() {
        const snowflake = document.createElement('div');
        snowflake.classList.add('snowflake');
        snowflake.style.left = Math.random() * 100 + 'vw';
        snowflake.style.animationDuration = Math.random() * 10 + 5 + 's';
        snowflake.innerText = '❄';
        document.body.appendChild(snowflake);
        snowflake.addEventListener('animationend', () => {
            snowflake.remove();
        });
    }
    setInterval(createSnowflake, 200);
})()