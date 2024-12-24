(function () {
    function createSnowflake() {
        const snowflake = document.createElement('div');
        snowflake.classList.add('snowflake');

        const size = Math.random() * 1.5 + 0.5;
        const left = Math.random() * 98 + 1;
        const duration = Math.random() * 10 + 5;

        snowflake.style.left = left + 'vw';
        snowflake.style.fontSize = size + 'em';
        snowflake.style.animationDuration = duration + 's';

        snowflake.innerText = '❄';
        document.body.appendChild(snowflake);
        snowflake.addEventListener('animationend', () => {
            snowflake.remove();
        });
    }

    function calculateInterval(width, baseInterval = 200, referenceWidth = 1000) {
        return (baseInterval * referenceWidth) / width;
    }

    let snowflakeInterval = setInterval(createSnowflake, calculateInterval(window.innerWidth));
    window.addEventListener('resize', () => {
        clearInterval(snowflakeInterval);
        snowflakeInterval = setInterval(createSnowflake, calculateInterval(window.innerWidth));
    });
})()