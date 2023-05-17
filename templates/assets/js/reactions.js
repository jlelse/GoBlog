(() => {
    const reactions = document.querySelector('#reactions');
    const path = reactions.dataset.path;
    const allowed = reactions.dataset.allowed.split(',');

    const updateCounts = async () => {
        try {
            const response = await fetch('/-/reactions?path=' + encodeURI(path));
            const json = await response.json();

            for (const reaction in json) {
                const button = document.querySelector(`#reactions button[data-reaction="${reaction}"]`);
                button.textContent = `${reaction} ${json[reaction]}`;
            }
        } catch (error) {
            console.error(error);
        }
    };

    const handleReactionClick = (allowedReaction) => {
        const data = new FormData();
        data.append('path', path);
        data.append('reaction', allowedReaction);

        return fetch('/-/reactions', { method: 'POST', body: data })
            .then(updateCounts)
            .catch((error) => {
                console.error(error);
            });
    };

    allowed.forEach((allowedReaction) => {
        const button = document.createElement('button');
        button.dataset.reaction = allowedReaction;
        button.addEventListener('click', () => handleReactionClick(allowedReaction));
        button.textContent = allowedReaction;
        reactions.appendChild(button);
    });

    (async () => {
        try {
            await updateCounts();
        } catch (error) {
            console.error(error);
        }
    })();
})();
