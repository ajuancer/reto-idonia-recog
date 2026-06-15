// Functionality for the PIN display
const pinDisplay = document.getElementById('pinDisplay');
const toggleBlur = document.getElementById('toggleBlur');
const copyBtn = document.getElementById('copyBtn');
const toastNotification = document.getElementById('toastNotification');
const toastText = toastNotification ? toastNotification.querySelector('[data-i18n]') : null;

// Variable to keep track of the timeout so we can reset it if the user clicks quickly
let toastTimeout;

toggleBlur.addEventListener('click', () => {
    pinDisplay.classList.toggle('blur-sm');
    pinDisplay.classList.toggle('select-none');
});

copyBtn.addEventListener('click', async () => {
    const textToCopy = pinDisplay.innerText;

    try {
        await navigator.clipboard.writeText(textToCopy);

        // Visual feedback on the button itself
        const originalColor = copyBtn.style.color;
        copyBtn.style.color = '#10b981'; // Tailwind emerald-500
        setTimeout(() => copyBtn.style.color = originalColor, 2000);

        showToast('toastCopied');

    } catch (err) {
        console.error('Failed to copy: ', err);
    }
});

// Helper function to animate the toast
function showToast(messageKey) {
    clearTimeout(toastTimeout);

    if (toastText && messageKey) {
        toastText.textContent = t(messageKey);
    }

    // Slide up and fade in
    toastNotification.classList.remove('translate-y-10', 'opacity-0');
    toastNotification.classList.add('translate-y-0', 'opacity-100');

    // Slide down and fade out after 3 seconds
    toastTimeout = setTimeout(() => {
        toastNotification.classList.remove('translate-y-0', 'opacity-100');
        toastNotification.classList.add('translate-y-10', 'opacity-0');
    }, 3000);
}

const shareBtn = document.getElementById('shareBtn');
const urlDisplay = document.getElementById('urlDisplay');

shareBtn.addEventListener('click', async () => {
    const pin = pinDisplay.innerText;
    const portalUrl = urlDisplay.getAttribute('href') !== '#' ? urlDisplay.href : window.location.href;

    if (navigator.share) {
        try {
            await navigator.share({
                title: t('shareTitle'),
                text: t('shareText', pin),
                url: portalUrl
            });
        } catch (err) {
            if (err.name !== 'AbortError') {
                console.error('Error sharing:', err);
            }
        }
    } else {
        try {
            await navigator.clipboard.writeText(`PIN: ${pin} | Link: ${portalUrl}`);
            showToast('toastShareFallback');
        } catch (copyErr) {
            console.error('Fallback copy failed: ', copyErr);
        }
    }
});
