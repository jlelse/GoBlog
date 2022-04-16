(function () {

    // Get reactions element
    let reactions = document.querySelector('#reactions')

    // Get post path
    let path = reactions.dataset.path

    // Define update counts function
    let updateCounts = function () {
        // Fetch reactions json
        fetch('/-/reactions?path=' + encodeURI(path))
            .then(response => response.json())
            .then(json => {
                // For every reaction
                for (let reaction in json) {
                    // Get reaction buttons
                    let button = document.querySelector('#reactions button[data-reaction="' + reaction + '"]')
                    // Set reaction count
                    button.innerText = reaction + ' ' + json[reaction]
                }
            })
    }

    // Get allowed reactions
    let allowed = reactions.dataset.allowed.split(',')
    allowed.forEach(allowedReaction => {

        // Create reaction button
        let button = document.createElement('button')
        button.dataset.reaction = allowedReaction

        // Set click event
        button.addEventListener('click', function () {
            // Send reaction to server
            let data = new FormData()
            data.append('path', path)
            data.append('reaction', allowedReaction)
            fetch('/-/reactions', { method: 'POST', body: data })
                .then(updateCounts)
        })

        // Set reaction text
        button.innerText = allowedReaction

        // Add button to reactions element
        reactions.appendChild(button)

    })

    // Update reaction counts
    updateCounts()

})()