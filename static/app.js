document.getElementById('searchButton').addEventListener('click', function () {
    const query = document.getElementById('searchInput').value;
    fetch(`/search?q=${encodeURIComponent(query)}`)
        .then(response => response.json())
        .then(data => {
            const resultsDiv = document.getElementById('results');
            resultsDiv.innerHTML = '';
            data.forEach(message => {
                const div = document.createElement('div');
                div.textContent = `${message.name}: ${message.text}`;
                div.dataset.id = message.id;
                div.addEventListener('click', function () {
                    fetch(`/messages/${message.id}`)
                        .then(response => response.json())
                        .then(data => {
                            const chatDiv = document.getElementById('chat');
                            chatDiv.innerHTML = '';
                            data.before_messages.forEach(msg => {
                                const msgDiv = document.createElement('div');
                                msgDiv.textContent = `${msg.name}: ${msg.text}`;
                                chatDiv.appendChild(msgDiv);
                            });
                            const selectedMessageDiv = document.createElement('div');
                            selectedMessageDiv.textContent = `${data.message.name}: ${data.message.text}`;
                            chatDiv.appendChild(selectedMessageDiv);
                            data.after_messages.forEach(msg => {
                                const msgDiv = document.createElement('div');
                                msgDiv.textContent = `${msg.name}: ${msg.text}`;
                                chatDiv.appendChild(msgDiv);
                            });
                            chatDiv.style.display = 'block';
                        });
                });
                resultsDiv.appendChild(div);
            });
        });
});

document.getElementById('chat').addEventListener('scroll', function () {
    const chatDiv = this;
    if (chatDiv.scrollTop === 0) {
        const firstMessageId = chatDiv.firstChild.dataset.id;
        fetch(`/messages/${firstMessageId}/before`)
            .then(response => response.json())
            .then(data => {
                data.forEach(msg => {
                    const msgDiv = document.createElement('div');
                    msgDiv.textContent = `${msg.name}: ${msg.text}`;
                    chatDiv.insertBefore(msgDiv, chatDiv.firstChild);
                });
            });
    } else if (chatDiv.scrollTop + chatDiv.clientHeight >= chatDiv.scrollHeight) {
        const lastMessageId = chatDiv.lastChild.dataset.id;
        fetch(`/messages/${lastMessageId}/after`)
            .then(response => response.json())
            .then(data => {
                data.forEach(msg => {
                    const msgDiv = document.createElement('div');
                    msgDiv.textContent = `${msg.name}: ${msg.text}`;
                    chatDiv.appendChild(msgDiv);
                });
            });
    }
});

