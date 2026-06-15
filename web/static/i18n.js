/**
 * i18n.js — lightweight internationalisation for the medical portal.
 *
 * Supported locales: en (English), es (Spanish).
 * Language is resolved from localStorage → navigator.language → 'en'.
 * Call t(key) anywhere to get the current locale's string.
 * Call setLocale(lang) to switch language at runtime.
 */

const TRANSLATIONS = {
    en: {
        // Page
        pageTitle:            'Secure Medical Portal | Document Upload',
        // Header
        welcome:              'Welcome!',
        subtitle:             'Upload your PDF and DICOM files.',
        // File picker
        clickToSelect:        'Click to select files',
        fileHelp:             'PDF or DICOM up to 100 MB',
        noFilesSelected:      'No files selected',
        // File status badges
        statusReady:          'Ready',
        statusTransferring:   'Transferring\u2026',
        statusProcessing:     'Processing\u2026',
        statusComplete:       'Complete',
        statusFailed:         'Failed',
        // Submit button
        btnUpload:            'Upload',
        btnTransferring:      'Transferring\u2026',
        // Processing panel
        processingTransfer:   (n) => `Transferring ${n} file${n === 1 ? '' : 's'} to server\u2026`,
        processingInit:       'Files transferred. Initialising background job\u2026',
        processingStatus:     'Generating report and uploading.',
        processingHint:       'Processing may take a minute. Please keep this tab open.',
        // Errors
        errNoFile:            'Please select a file.',
        errAlreadyProcessing: 'This request is already processing. Please wait.',
        errServerRejected:    'Server rejected the upload.',
        errNoJobId:           'No Job ID returned from the server.',
        errGeneric:           'An error occurred during upload. Please try again.',
        errPollFailed:        'Failed to fetch job status',
        errBgFailed:          'Background processing failed. Please try again.',
        // Result section
        resultTitle:          'Upload Successful',
        resultSubtitle:       'Your files have been processed securely.',
        pinLabel:             'Access PIN',
        btnCopyPin:           'Copy PIN',
        btnShare:             'Share',
        linkPortal:           'Go to Idonia portal',
        // Toast
        toastCopied:          'PIN copied to clipboard!',
        toastShareFallback:   'Sharing not supported. Info copied to clipboard!',
        // Share sheet
        shareTitle:           'Secure Medical Portal Access',
        shareText:            (pin) => `Your access PIN for the medical documents is: ${pin}`,
        // Language toggle
        langToggleLabel:      'Español',
    },

    es: {
        // Page
        pageTitle:            'Portal Médico Seguro | Subida de Documentos',
        // Header
        welcome:              '¡Bienvenido!',
        subtitle:             'Sube tus archivos PDF y DICOM.',
        // File picker
        clickToSelect:        'Haz clic para seleccionar archivos',
        fileHelp:             'PDF o DICOM hasta 100 MB',
        noFilesSelected:      'Ningún archivo seleccionado',
        // File status badges
        statusReady:          'Listo',
        statusTransferring:   'Transfiriendo\u2026',
        statusProcessing:     'Procesando\u2026',
        statusComplete:       'Completado',
        statusFailed:         'Error',
        // Submit button
        btnUpload:            'Subir',
        btnTransferring:      'Transfiriendo\u2026',
        // Processing panel
        processingTransfer:   (n) => `Transfiriendo ${n} archivo${n === 1 ? '' : 's'} al servidor\u2026`,
        processingInit:       'Archivos transferidos. Iniciando tarea en segundo plano\u2026',
        processingStatus:     'Generando informe y subiendo.',
        processingHint:       'El procesamiento puede tardar un minuto. Por favor, mantén esta pestaña abierta.',
        // Errors
        errNoFile:            'Por favor, selecciona un archivo.',
        errAlreadyProcessing: 'Esta solicitud ya está en proceso. Por favor, espera.',
        errServerRejected:    'El servidor rechazó la subida.',
        errNoJobId:           'El servidor no devolvió un ID de tarea.',
        errGeneric:           'Ocurrió un error durante la subida. Por favor, inténtalo de nuevo.',
        errPollFailed:        'No se pudo obtener el estado de la tarea',
        errBgFailed:          'El procesamiento en segundo plano falló. Por favor, inténtalo de nuevo.',
        // Result section
        resultTitle:          'Subida Exitosa',
        resultSubtitle:       'Tus archivos han sido procesados de forma segura.',
        pinLabel:             'PIN de Acceso',
        btnCopyPin:           'Copiar PIN',
        btnShare:             'Compartir',
        linkPortal:           'Ir al portal de Idonia',
        // Toast
        toastCopied:          '¡PIN copiado al portapapeles!',
        toastShareFallback:   'Compartir no disponible. ¡Información copiada al portapapeles!',
        // Share sheet
        shareTitle:           'Acceso al Portal Médico Seguro',
        shareText:            (pin) => `Tu PIN de acceso para los documentos médicos es: ${pin}`,
        // Language toggle
        langToggleLabel:      'English',
    },
};

/** Resolve the initial locale: localStorage → browser → 'en'. */
function resolveLocale() {
    const stored = localStorage.getItem('locale');
    if (stored && TRANSLATIONS[stored]) return stored;

    const browser = (navigator.language || '').slice(0, 2).toLowerCase();
    return TRANSLATIONS[browser] ? browser : 'en';
}

let currentLocale = resolveLocale();

/** Return the translation for key in the current locale. */
function t(key, ...args) {
    const entry = TRANSLATIONS[currentLocale]?.[key] ?? TRANSLATIONS['en']?.[key];
    if (typeof entry === 'function') return entry(...args);
    return entry ?? key;
}

/** Switch locale, persist it, and re-render all data-i18n nodes. */
function setLocale(lang) {
    if (!TRANSLATIONS[lang]) return;
    currentLocale = lang;
    localStorage.setItem('locale', lang);
    applyTranslations();
}

/** Apply translations to every element carrying a data-i18n attribute. */
function applyTranslations() {
    document.querySelectorAll('[data-i18n]').forEach((el) => {
        const key = el.getAttribute('data-i18n');
        el.textContent = t(key);
    });

    // Update <html lang>
    document.documentElement.lang = currentLocale;

    // Update <title>
    document.title = t('pageTitle');

    // Update the language toggle button label
    const btn = document.getElementById('langToggleBtn');
    if (btn) btn.textContent = t('langToggleLabel');
}

/** Inject the language toggle button into the given container element. */
function injectLangToggle(container) {
    const btn = document.createElement('button');
    btn.id = 'langToggleBtn';
    btn.type = 'button';
    btn.textContent = t('langToggleLabel');
    btn.className = [
        'absolute top-4 right-4',
        'text-xs font-semibold text-blue-600 hover:text-blue-800',
        'border border-blue-200 hover:border-blue-400',
        'rounded-full px-3 py-1 transition-colors',
        'focus:outline-none focus:ring-2 focus:ring-blue-300',
    ].join(' ');
    btn.setAttribute('aria-label', 'Switch language');
    btn.addEventListener('click', () => {
        setLocale(currentLocale === 'en' ? 'es' : 'en');
    });
    container.appendChild(btn);
}

// Bootstrap on DOMContentLoaded
document.addEventListener('DOMContentLoaded', () => {
    const main = document.querySelector('main');
    if (main) {
        // Make <main> the positioning parent for the toggle button
        main.style.position = 'relative';
        injectLangToggle(main);
    }
    applyTranslations();
});
