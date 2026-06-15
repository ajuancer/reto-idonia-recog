const fileInput = document.getElementById('fileInput');
const fileList = document.getElementById('fileList');
const form = document.getElementById('uploadForm');
const submitBtn = document.getElementById('submitBtn');
const resultContainer = document.getElementById('resultContainer');
const processingStatus = document.getElementById('processingStatus');
const processingText = document.getElementById('processingText');
const processingHint = document.getElementById('processingHint');
const errorMessage = document.getElementById('errorMessage');

let fileStatusNodes = [];

const statusClasses = {
    ready: 'text-gray-400',
    uploading: 'text-blue-600',
    done: 'text-green-600',
    failed: 'text-red-600'
};

const buildStatusClass = (statusClass) => `text-xs font-semibold ${statusClass}`;

const createIdempotencyKey = () => {
    if (window.crypto && window.crypto.randomUUID) {
        return window.crypto.randomUUID();
    }
    if (window.crypto && window.crypto.getRandomValues) {
        const bytes = new Uint8Array(16);
        window.crypto.getRandomValues(bytes);
        return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('');
    }
    return `${Date.now()}-${Math.random().toString(16).slice(2)}`;
};

const showError = (message) => {
    if (!errorMessage) return;
    errorMessage.textContent = message;
    errorMessage.classList.remove('hidden');
};

const hideError = () => {
    if (!errorMessage) return;
    errorMessage.classList.add('hidden');
    errorMessage.textContent = '';
};

const renderFileList = (files) => {
    fileList.innerHTML = '';
    fileStatusNodes = [];

    if (files.length === 0) {
        fileList.textContent = t('noFilesSelected');
        return;
    }

    files.forEach((file) => {
        const row = document.createElement('div');
        row.className = 'flex items-center justify-between rounded-md bg-white px-3 py-2 border border-gray-100 shadow-sm';

        const name = document.createElement('span');
        name.className = 'truncate pr-2 text-gray-700';
        name.textContent = file.name;

        const status = document.createElement('span');
        status.className = buildStatusClass(statusClasses.ready);
        status.textContent = t('statusReady');

        row.append(name, status);
        fileList.appendChild(row);
        fileStatusNodes.push(status);
    });
};

const updateFileStatuses = (textKey, statusClass, ...args) => {
    const text = t(textKey, ...args);
    fileStatusNodes.forEach((status) => {
        status.textContent = text;
        status.className = buildStatusClass(statusClass);
    });
};

const setProcessingState = (isProcessing, messageKey, ...args) => {
    if (messageKey) {
        processingText.textContent = t(messageKey, ...args);
    }
    if (processingHint) {
        processingHint.textContent = t('processingHint');
    }
    processingStatus.classList.toggle('hidden', !isProcessing);
    form.setAttribute('aria-busy', isProcessing ? 'true' : 'false');
    fileInput.disabled = isProcessing;
    submitBtn.disabled = isProcessing;
    submitBtn.classList.toggle('opacity-75', isProcessing);
    submitBtn.classList.toggle('cursor-not-allowed', isProcessing);
};

// Polling function to check background job status
const pollJobStatus = async (jobId) => {
    try {
        const response = await fetch(`/api/v1/upload/status/${jobId}`);
        if (!response.ok) throw new Error(t('errPollFailed'));

        const job = await response.json();

        if (job.status === 'completed') {
            updateFileStatuses('statusComplete', statusClasses.done);
            form.classList.add('hidden');
            resultContainer.classList.remove('hidden');

            document.getElementById('pinDisplay').textContent = job.result.pin;
            document.getElementById('urlDisplay').href = job.result.viewer_url;

            submitBtn.querySelector('[data-i18n]').textContent = t('btnUpload');
            setProcessingState(false);

        } else if (job.status === 'failed') {
            throw new Error(job.error || t('errBgFailed'));

        } else {
            setProcessingState(true, 'processingStatus');
            setTimeout(() => pollJobStatus(jobId), 3000);
        }
    } catch (error) {
        updateFileStatuses('statusFailed', statusClasses.failed);
        showError(error.message);
        console.error('Polling error:', error);

        submitBtn.querySelector('[data-i18n]').textContent = t('btnUpload');
        setProcessingState(false);
    }
};

// Show selected files and hide any existing errors
fileInput.addEventListener('change', (e) => {
    hideError();
    renderFileList(Array.from(e.target.files));
});

// Handle submission
form.addEventListener('submit', async (e) => {
    e.preventDefault();
    hideError();

    const files = Array.from(fileInput.files);

    if (files.length === 0) {
        showError(t('errNoFile'));
        return;
    }

    const formData = new FormData();
    for (const file of files) {
        formData.append('files', file);
    }

    const fileCount = files.length;

    submitBtn.querySelector('[data-i18n]').textContent = t('btnTransferring');
    updateFileStatuses('statusTransferring', statusClasses.uploading);
    setProcessingState(true, 'processingTransfer', fileCount);

    const csrfToken = document.querySelector('meta[name="csrf-token"]').getAttribute('content');
    const idempotencyKey = createIdempotencyKey();

    try {
        const response = await fetch('/api/v1/upload', {
            method: 'POST',
            headers: {
                'X-CSRF-Token': csrfToken,
                'Idempotency-Key': idempotencyKey
            },
            body: formData
        });

        if (!response.ok) {
            if (response.status === 409) {
                throw new Error(t('errAlreadyProcessing'));
            }
            throw new Error(t('errServerRejected'));
        }

        const data = await response.json();

        if (data.job_id) {
            updateFileStatuses('statusProcessing', statusClasses.uploading);
            setProcessingState(true, 'processingInit');
            pollJobStatus(data.job_id);
        } else {
            throw new Error(t('errNoJobId'));
        }

    } catch (error) {
        updateFileStatuses('statusFailed', statusClasses.failed);
        showError(error.message || t('errGeneric'));
        console.error(error);

        submitBtn.querySelector('[data-i18n]').textContent = t('btnUpload');
        setProcessingState(false);
    }
});
