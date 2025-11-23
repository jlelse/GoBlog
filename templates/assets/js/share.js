(() => {
    const shareBtn = document.getElementById('shareBtn');
    const shareModal = document.getElementById('shareModal');
    const shareDataElement = document.getElementById('shareData');
    if (!shareBtn || !shareModal || !shareDataElement) {
        return;
    }

    let shareData = null;
    try {
        const raw = shareDataElement.textContent?.trim();
        shareData = raw ? JSON.parse(raw) : null;
    } catch (error) {
        console.error('Failed to read share data', error);
        return;
    }

    if (!shareData) {
        return;
    }

    const shareTitle = shareData.title || '';
    const shareUrl = shareData.url || '';
    const shareText = shareData.text || '';
    const shareServices = Array.isArray(shareData.services) ? shareData.services : [];
    const supportsNativeShare = typeof navigator.share === 'function';

    const overlay = document.getElementById('shareModalOverlay');
    const servicesContainer = document.getElementById('shareModalServices');
    const copyButton = document.getElementById('shareModalCopy');
    const nativeShareButton = document.getElementById('shareModalNative');
    const closeTriggers = shareModal.querySelectorAll('.share-modal-close');

    shareServices.forEach((service) => {
        if (!servicesContainer || !service) {
            return;
        }
        const link = document.createElement('a');
        link.className = 'button';
        link.setAttribute('href', service.url);
        link.setAttribute('target', '_blank');
        link.setAttribute('rel', 'noopener noreferrer');
        link.textContent = service.label || service.id;
        servicesContainer.appendChild(link);
    });

    shareBtn.addEventListener('click', () => {
        shareModal.classList.remove('hide');
    });

    const closeAll = () => {
        if (!shareModal.classList.contains('hide')) {
            shareModal.classList.add('hide');
            shareBtn.focus();
        }
    };

    overlay?.addEventListener('click', closeAll);
    closeTriggers.forEach((trigger) => trigger.addEventListener('click', closeAll));
    document.addEventListener('keydown', (event) => {
        if (event.key === 'Escape') {
            closeAll();
        }
    });

    if (supportsNativeShare) {
        nativeShareButton.classList.remove('hide');
        nativeShareButton.addEventListener('click', async () => {
            try {
                await navigator.share({
                    text: shareTitle,
                    url: shareUrl,
                });
                closeAll();
            } catch (error) {
                console.error('Share API failed', error);
            }
        });
    }

    copyButton?.addEventListener('click', async () => {
        try {
            await navigator.clipboard.writeText(shareText);
            copyButton.textContent = copyButton.dataset.shareCopyFeedback;
        } catch (error) {
            console.error('Failed to copy share text', error);
        }
    });
})();
