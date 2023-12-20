(async () => {
    const reactions = document.querySelector('#reactions');
    const path = reactions.dataset.path;
    const allowed = reactions.dataset.allowed.split(',');

    // Function to update reaction counts
    const updateCounts = async () => {
        try {
            const response = await fetch(`/-/reactions?path=${encodeURI(path)}`);
            const json = await response.json();

            for (const reaction in json) {
                const button = document.querySelector(`#reactions button[data-reaction="${reaction}"]`);
                button.textContent = `${reaction} ${json[reaction]}`;
            }
        } catch (error) {
            console.error('Error updating counts:', error);
        }
    };

    // Function to handle reaction click
    const handleReactionClick = async (allowedReaction) => {
        const data = new FormData();
        data.append('path', path);
        data.append('reaction', allowedReaction);

        try {
            await fetch('/-/reactions', { method: 'POST', body: data });
            await updateCounts();
        } catch (error) {
            console.error('Error handling reaction click:', error);
        }
    };

    // Create buttons for each allowed reaction
    allowed.forEach((allowedReaction) => {
        const button = document.createElement('button');
        button.dataset.reaction = allowedReaction;
        button.addEventListener('click', () => {
            handleReactionClick(allowedReaction).then(() => {});
        });
        button.textContent = allowedReaction;
        reactions.appendChild(button);
    });

    // Initial update of counts
    try {
        await updateCounts();
    } catch (error) {
        console.error('Error during initial update:', error);
    }
})();