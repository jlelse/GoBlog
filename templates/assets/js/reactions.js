(async () => {
    const reactions = document.querySelector('#reactions');
    const path = reactions.dataset.path;
    const allowed = reactions.dataset.allowed.split(',');

    // Function to update reaction counts
    const updateButtonCounts = (counts) => {
        for (const reaction in counts) {
            const button = document.querySelector(`#reactions button[data-reaction="${reaction}"]`);
            if (button) {
                button.textContent = `${reaction} ${counts[reaction]}`;
            }
        }
    };

    // Function to handle reaction click
    const handleReactionClick = async (allowedReaction) => {
        const button = document.querySelector(`#reactions button[data-reaction="${allowedReaction}"]`);
        button.disabled = true;
        try {
            const data = new FormData();
            data.append('path', path);
            data.append('reaction', allowedReaction);
            const response = await fetch('/-/reactions', { method: 'POST', body: data });
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            const updatedCounts = await response.json();
            updateButtonCounts(updatedCounts);
        } catch (error) {
            console.error('Error handling reaction click:', error);
        } finally {
            button.disabled = false;
        }
    };

    // Create buttons for each allowed reaction
    allowed.forEach((allowedReaction) => {
        const button = document.createElement('button');
        button.dataset.reaction = allowedReaction;
        button.addEventListener('click', () => {
            handleReactionClick(allowedReaction);
        });
        button.textContent = allowedReaction;
        reactions.appendChild(button);
    });

    // Initial update of counts
    try {
        const response = await fetch(`/-/reactions?path=${encodeURI(path)}`);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const initialCounts = await response.json();
        updateButtonCounts(initialCounts);
    } catch (error) {
        console.error('Error during initial update:', error);
    }
})();