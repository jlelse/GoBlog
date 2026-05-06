(() => {
    const shareBtn = document.getElementById('shareBtn');
    const shareModal = document.getElementById('shareModal');
    const shareDataElement = document.getElementById('shareData');
    if (!shareBtn || !shareModal || !shareDataElement) return;

    let shareData;
    try {
        shareData = JSON.parse(shareDataElement.textContent);
    } catch (error) {
        console.error('Failed to read share data', error);
        return;
    }

    const { title = '', url = '', text = '', services = [] } = shareData || {};

    const servicesContainer = document.getElementById('shareModalServices');
    const copyButton = document.getElementById('shareModalCopy');
    const nativeShareButton = document.getElementById('shareModalNative');
    const closeButton = shareModal.querySelector('.share-modal-close');

    services.forEach((service) => {
        const link = document.createElement('a');
        link.className = 'button';
        link.href = service.url;
        link.target = '_blank';
        link.rel = 'noopener noreferrer';
        link.textContent = service.label || service.id;
        servicesContainer.appendChild(link);
    });

    shareBtn.addEventListener('click', () => {
        shareModal.showModal();
        shareModal.focus();
    });
    closeButton.addEventListener('click', () => shareModal.close());
    shareModal.addEventListener('click', (event) => {
        if (event.target === shareModal) shareModal.close();
    });

    if (typeof navigator.share === 'function') {
        nativeShareButton.classList.remove('hide');
        nativeShareButton.addEventListener('click', async () => {
            try {
                await navigator.share({ text: title, url });
                shareModal.close();
            } catch (error) {
                console.error('Share API failed', error);
            }
        });
    }

    copyButton.addEventListener('click', async () => {
        try {
            await navigator.clipboard.writeText(text);
            copyButton.textContent = copyButton.dataset.shareCopyFeedback;
        } catch (error) {
            console.error('Failed to copy share text', error);
        }
    });
})();
