// Wails Frontend - Predator YouTube Downloader
import {
    IsValidYouTubeURL,
    FetchVideoInfo,
    AddToQueue,
    CancelTask,
    SelectOutputDir,
    GetOutputDir,
    CheckAndInstallDeps,
    GetDownloadHistory,
    OpenFolder,
    ClearHistory
} from '../wailsjs/go/main/App';
import {EventsOn, EventsEmit} from '../wailsjs/runtime/runtime';
import lightIcon from './assets/images/summer.png';
import darkIcon from './assets/images/dark-mode.png';

// DOM Elements
const urlInput = document.getElementById('url-input');
const clearBtn = document.getElementById('clear-btn');
const titleDisplay = document.getElementById('title-display');
const downloadTypeRadios = document.querySelectorAll('input[name="download-type"]');
const resolutionSelect = document.getElementById('resolution-select');
const audioFormatSelect = document.getElementById('audio-format-select');
const resolutionWrapper = document.getElementById('resolution-wrapper');
const audioFormatWrapper = document.getElementById('audio-format-wrapper');
const addBtn = document.getElementById('add-btn');
const statusLabel = document.getElementById('status-label');
const outputDirDisplay = document.getElementById('output-dir-display');
const changeDirBtn = document.getElementById('change-dir-btn');
const themeToggle = document.getElementById('theme-toggle');
const themeIcon = document.getElementById('theme-icon');
const tabBtns = document.querySelectorAll('.tab-btn');
const tabContents = document.querySelectorAll('.tab-content');
const downloadsContainer = document.getElementById('downloads-container');
const historyContainer = document.getElementById('history-container');
const filterBtns = document.querySelectorAll('.filter-btn');
const clearHistoryModal = document.getElementById('clear-history-modal');
const modalCloseBtn = document.getElementById('modal-close-btn');
const modalCancelBtn = document.getElementById('modal-cancel-btn');
const modalConfirmBtn = document.getElementById('modal-confirm-btn');

// File Not Found Modal Elements
const fileNotFoundModal = document.getElementById('file-not-found-modal');
const fileNotFoundCloseBtn = document.getElementById('file-not-found-close-btn');
const fileNotFoundOkBtn = document.getElementById('file-not-found-ok-btn');
const fileNotFoundMessage = document.getElementById('file-not-found-message');

// State
let currentVideoInfo = null;
let fetchTimer = null;
let isFetching = false;
let isDark = true;
let activeDownloads = new Map();
let downloadHistory = [];
let currentFilter = 'all';

// Constants
const FETCH_DEBOUNCE = 600;
const MAX_RETRIES = 3;

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
    // Check dependencies on startup
    try {
        await CheckAndInstallDeps();
    } catch (err) {
        console.log('Dependency check:', err);
    }

    // Get initial output directory
    updateOutputDir();

    // Setup event listeners
    setupEventListeners();

    // Setup Wails event handlers
    setupWailsEvents();
});

// Event Listeners
function setupEventListeners() {
    // URL input with debounce
    urlInput.addEventListener('input', handleUrlInput);

    // Enable right-click context menu for paste
    urlInput.addEventListener('contextmenu', (e) => {
        // Allow default browser context menu for paste functionality
        e.stopPropagation();
    });

    // Clear button
    clearBtn.addEventListener('click', () => {
        urlInput.value = '';
        resetUI();
    });

    // Download type change
    downloadTypeRadios.forEach(radio => {
        radio.addEventListener('change', handleDownloadTypeChange);
    });

    // Add to queue
    addBtn.addEventListener('click', addToQueue);

    // Change output directory
    changeDirBtn.addEventListener('click', async () => {
        try {
            const dir = await SelectOutputDir();
            if (dir) {
                updateOutputDir();
            }
        } catch (err) {
            console.error('Failed to select directory:', err);
        }
    });

    // Theme toggle
    themeToggle.addEventListener('click', toggleTheme);

    // Tab switching
    tabBtns.forEach(btn => {
        btn.addEventListener('click', () => switchTab(btn.dataset.tab));
    });

    // Filter buttons
    filterBtns.forEach(btn => {
        btn.addEventListener('click', () => setFilter(btn.dataset.filter));
    });

    // Clear history button
    const clearHistoryBtn = document.getElementById('clear-history-btn');
    if (clearHistoryBtn) {
        clearHistoryBtn.addEventListener('click', showClearHistoryModal);
    }

    // Modal event listeners
    if (modalCloseBtn) {
        modalCloseBtn.addEventListener('click', hideClearHistoryModal);
    }
    if (modalCancelBtn) {
        modalCancelBtn.addEventListener('click', hideClearHistoryModal);
    }
    if (modalConfirmBtn) {
        modalConfirmBtn.addEventListener('click', confirmClearHistory);
    }
    if (clearHistoryModal) {
        clearHistoryModal.addEventListener('click', (e) => {
            if (e.target === clearHistoryModal) {
                hideClearHistoryModal();
            }
        });
    }

    // File Not Found Modal event listeners
    if (fileNotFoundCloseBtn) {
        fileNotFoundCloseBtn.addEventListener('click', hideFileNotFoundModal);
    }
    if (fileNotFoundOkBtn) {
        fileNotFoundOkBtn.addEventListener('click', hideFileNotFoundModal);
    }
    if (fileNotFoundModal) {
        fileNotFoundModal.addEventListener('click', (e) => {
            if (e.target === fileNotFoundModal) {
                hideFileNotFoundModal();
            }
        });
    }
}

// Wails Event Handlers
function setupWailsEvents() {
    // Task started
    EventsOn('task-started', (taskId, task) => {
        createDownloadTask(taskId, task);
    });

    // Task progress
    EventsOn('task-progress', (update) => {
        updateDownloadProgress(update);
    });

    // Task retry
    EventsOn('task-retry', (taskId, attempt, maxAttempts) => {
        updateTaskStatus(taskId, `Retrying... (${attempt}/${maxAttempts})`);
    });

    // Task completed
    EventsOn('task-completed', (taskId, task) => {
        completeDownloadTask(taskId);
        // Refresh history after download completes
        setTimeout(() => loadHistory(), 1000);
    });

    // Task error
    EventsOn('task-error', (taskId, error) => {
        errorDownloadTask(taskId, error);
    });

    // Task cancelled
    EventsOn('task-cancelled', (taskId) => {
        cancelDownloadTask(taskId);
    });

    // Installing deps
    EventsOn('installing-deps', (installing) => {
        if (installing) {
            statusLabel.textContent = 'Installing dependencies...';
        } else {
            statusLabel.textContent = 'Ready';
        }
    });
}

// URL Input Handler with Debounce
function handleUrlInput() {
    const url = urlInput.value.trim();

    // Clear existing timer
    if (fetchTimer) {
        clearTimeout(fetchTimer);
    }

    // Reset UI if empty
    if (!url) {
        resetUI();
        return;
    }

    // Validate URL
    if (!IsValidYouTubeURL(url)) {
        statusLabel.textContent = 'Invalid YouTube URL';
        titleDisplay.classList.add('hidden');
        resolutionSelect.disabled = true;
        addBtn.disabled = true;
        return;
    }

    // Debounced fetch
    fetchTimer = setTimeout(async () => {
        if (isFetching) return;
        isFetching = true;

        statusLabel.textContent = 'Fetching video info...';
        resolutionSelect.disabled = true;
        addBtn.disabled = true;

        try {
            const info = await FetchVideoInfo(url);
            currentVideoInfo = info;
            displayVideoInfo(info);
            statusLabel.textContent = 'Ready to download';
        } catch (err) {
            console.error('Fetch error:', err);
            statusLabel.textContent = 'Failed to fetch info';
            titleDisplay.classList.add('hidden');
        } finally {
            isFetching = false;
        }
    }, FETCH_DEBOUNCE);
}

// Display Video Info
function displayVideoInfo(info) {
    titleDisplay.textContent = `Title: ${info.title}`;
    titleDisplay.classList.remove('hidden');

    // Populate resolution select
    resolutionSelect.innerHTML = '';
    info.resolutions.forEach(res => {
        const option = document.createElement('option');
        option.value = res;
        option.textContent = res;
        resolutionSelect.appendChild(option);
    });

    // Auto-select best quality (prefer 1080p or 720p)
    const bestIndex = info.resolutions.findIndex(r => 
        r.includes('1080p') || r.includes('720p')
    );
    if (bestIndex >= 0) {
        resolutionSelect.selectedIndex = bestIndex;
    }

    resolutionSelect.disabled = false;
    addBtn.disabled = false;
}

// Handle Download Type Change
function handleDownloadTypeChange(e) {
    const type = e.target.value;

    if (type === 'Video') {
        resolutionWrapper.classList.remove('hidden');
        audioFormatWrapper.classList.add('hidden');
        resolutionSelect.disabled = !currentVideoInfo;
    } else {
        resolutionWrapper.classList.add('hidden');
        audioFormatWrapper.classList.remove('hidden');
    }
}

// Add to Queue
async function addToQueue() {
    const url = urlInput.value.trim();
    const type = document.querySelector('input[name="download-type"]:checked').value;

    if (!url || !await IsValidYouTubeURL(url)) {
        statusLabel.textContent = 'Please enter a valid YouTube URL';
        return;
    }

    const task = {
        url: url,
        title: currentVideoInfo?.title || 'Unknown',
        type: type,
        resolution: type === 'Video' ? resolutionSelect.value : '',
        cleanRes: type === 'Video' ? extractResolution(resolutionSelect.value) : '',
        audioFormat: type === 'Audio' ? audioFormatSelect.value : '',
        audioQuality: '0',
        videoCodec: ''
    };

    try {
        const taskId = await AddToQueue(task);
        console.log('Added to queue:', taskId);

        // Reset UI
        urlInput.value = '';
        resetUI();

        // Switch to history tab
        switchTab('history');

        statusLabel.textContent = 'Added to queue';
    } catch (err) {
        console.error('Failed to add to queue:', err);
        statusLabel.textContent = 'Failed to add to queue';
    }
}

// Create Download Task UI
function createDownloadTask(taskId, task) {
    const taskElement = document.createElement('div');
    taskElement.className = 'download-task';
    taskElement.id = `task-${taskId}`;
    taskElement.innerHTML = `
        <div class="task-header">
            <div class="task-title">${escapeHtml(task.title)}</div>
            <button class="task-cancel-btn" onclick="cancelTask('${taskId}')">Cancel</button>
        </div>
        <div class="progress-container">
            <div class="progress-bar">
                <div class="progress-fill" style="width: 0%"></div>
            </div>
            <div class="progress-info">
                <span class="progress-percent">0%</span>
                <span class="progress-speed"></span>
            </div>
        </div>
        <div class="task-status">Starting...</div>
    `;

    // Remove empty state if present
    const emptyState = downloadsContainer.querySelector('.empty-state');
    if (emptyState) {
        emptyState.remove();
    }

    downloadsContainer.appendChild(taskElement);
    activeDownloads.set(taskId, taskElement);
}

// Update Download Progress
function updateDownloadProgress(update) {
    const taskElement = activeDownloads.get(update.taskId);
    if (!taskElement) return;

    const progressFill = taskElement.querySelector('.progress-fill');
    const progressPercent = taskElement.querySelector('.progress-percent');
    const progressSpeed = taskElement.querySelector('.progress-speed');
    const taskStatus = taskElement.querySelector('.task-status');

    const percent = Math.min(update.percent, 100);
    progressFill.style.width = `${percent}%`;
    progressPercent.textContent = `${percent.toFixed(1)}%`;

    if (update.status === 'downloading') {
        progressSpeed.textContent = `${update.speed} | ETA: ${update.eta}`;
        taskStatus.textContent = `Downloading... ${percent.toFixed(1)}%`;
        taskStatus.className = 'task-status';
    } else if (update.status === 'processing') {
        progressSpeed.textContent = '';
        taskStatus.textContent = 'Processing... (merging)';
    }
}

// Update Task Status
function updateTaskStatus(taskId, status) {
    const taskElement = activeDownloads.get(taskId);
    if (!taskElement) return;

    const taskStatus = taskElement.querySelector('.task-status');
    taskStatus.textContent = status;
}

// Complete Download Task
function completeDownloadTask(taskId) {
    const taskElement = activeDownloads.get(taskId);
    if (!taskElement) return;

    const progressFill = taskElement.querySelector('.progress-fill');
    const progressPercent = taskElement.querySelector('.progress-percent');
    const progressSpeed = taskElement.querySelector('.progress-speed');
    const taskStatus = taskElement.querySelector('.task-status');
    const cancelBtn = taskElement.querySelector('.task-cancel-btn');

    progressFill.style.width = '100%';
    progressPercent.textContent = '100%';
    progressSpeed.textContent = '';
    taskStatus.textContent = 'Completed ✓';
    taskStatus.className = 'task-status completed';
    cancelBtn.disabled = true;

    // Remove after delay
    setTimeout(() => {
        taskElement.remove();
        activeDownloads.delete(taskId);
        checkEmptyState();
    }, 3000);
}

// Load Download History
async function loadHistory() {
    try {
        downloadHistory = await GetDownloadHistory();
        renderHistory();
    } catch (err) {
        console.error('Failed to load history:', err);
    }
}

// Set Filter
function setFilter(filter) {
    currentFilter = filter;
    
    // Update button states
    filterBtns.forEach(btn => {
        btn.classList.toggle('active', btn.dataset.filter === filter);
    });
    
    renderHistory();
}

// Render History
function renderHistory() {
    // Filter history
    let filtered = downloadHistory;
    if (currentFilter !== 'all') {
        filtered = downloadHistory.filter(item => item.type === currentFilter);
    }
    
    // Clear container
    historyContainer.innerHTML = '';
    
    if (filtered.length === 0) {
        historyContainer.innerHTML = '<div class="empty-state">No download history</div>';
        return;
    }
    
    // Render items
    filtered.forEach(item => {
        const historyItem = createHistoryItem(item);
        historyContainer.appendChild(historyItem);
    });
}

// Create History Item Element
function createHistoryItem(item) {
    const div = document.createElement('div');
    div.className = 'history-item';
    
    const isVideo = item.type === 'Video';
    const icon = isVideo ? '🎬' : '🎵';
    const typeClass = isVideo ? 'video' : 'audio';
    const format = isVideo ? item.resolution : item.audioFormat;
    const size = item.fileSize ? formatBytes(item.fileSize) : 'Unknown size';
    const date = new Date(item.downloadedAt).toLocaleDateString();
    
    div.innerHTML = `
        <div class="history-item-icon ${typeClass}">${icon}</div>
        <div class="history-item-info">
            <div class="history-item-title">${escapeHtml(item.title)}</div>
            <div class="history-item-meta">
                <span>${format}</span>
                <span class="history-item-size">${size}</span>
                <span>${date}</span>
            </div>
        </div>
        <button class="history-item-folder-btn" title="Open folder" onclick="openHistoryFolder('${escapeHtml(item.filePath)}')">
            📁
        </button>
    `;
    
    return div;
}

// Open History Folder
window.openHistoryFolder = async function(path) {
    try {
        // Decode HTML entities that might have been encoded by escapeHtml
        const decodedPath = path.replace(/&amp;/g, '&')
                               .replace(/</g, '<')
                               .replace(/>/g, '>')
                               .replace(/"/g, '"')
                               .replace(/&#039;/g, "'");
        
        console.log('Opening folder:', decodedPath);
        await OpenFolder(decodedPath);
    } catch (err) {
        console.error('Failed to open folder:', err);
        showFileNotFoundModal('The file or folder could not be found. It may have been moved or deleted.');
    }
};

// Show File Not Found Modal
function showFileNotFoundModal(message) {
    if (fileNotFoundMessage) {
        fileNotFoundMessage.textContent = message || 'The downloaded file could not be found.';
    }
    if (fileNotFoundModal) {
        fileNotFoundModal.classList.add('active');
    }
}

// Hide File Not Found Modal
function hideFileNotFoundModal() {
    if (fileNotFoundModal) {
        fileNotFoundModal.classList.remove('active');
    }
}

// Format bytes helper
function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

// Show Clear History Modal
function showClearHistoryModal() {
    if (clearHistoryModal) {
        clearHistoryModal.classList.add('active');
    }
}

// Hide Clear History Modal
function hideClearHistoryModal() {
    if (clearHistoryModal) {
        clearHistoryModal.classList.remove('active');
    }
}

// Confirm Clear History
async function confirmClearHistory() {
    hideClearHistoryModal();
    
    try {
        await ClearHistory();
        downloadHistory = [];
        renderHistory();
        console.log('History cleared');
    } catch (err) {
        console.error('Failed to clear history:', err);
    }
}

// Error Download Task
function errorDownloadTask(taskId, error) {
    const taskElement = activeDownloads.get(taskId);
    if (!taskElement) return;

    const progressSpeed = taskElement.querySelector('.progress-speed');
    const taskStatus = taskElement.querySelector('.task-status');
    const cancelBtn = taskElement.querySelector('.task-cancel-btn');

    progressSpeed.textContent = '';
    taskStatus.textContent = error;
    taskStatus.className = 'task-status error';
    cancelBtn.disabled = true;

    // Remove after delay
    setTimeout(() => {
        taskElement.remove();
        activeDownloads.delete(taskId);
        checkEmptyState();
    }, 5000);
}

// Cancel Download Task
function cancelDownloadTask(taskId) {
    const taskElement = activeDownloads.get(taskId);
    if (!taskElement) return;

    const progressSpeed = taskElement.querySelector('.progress-speed');
    const taskStatus = taskElement.querySelector('.task-status');
    const cancelBtn = taskElement.querySelector('.task-cancel-btn');

    progressSpeed.textContent = '';
    taskStatus.textContent = 'Cancelled';
    taskStatus.className = 'task-status cancelled';
    cancelBtn.disabled = true;

    // Remove after delay
    setTimeout(() => {
        taskElement.remove();
        activeDownloads.delete(taskId);
        checkEmptyState();
    }, 2000);
}

// Cancel Task (called from UI)
window.cancelTask = async function(taskId) {
    try {
        await CancelTask(taskId);
    } catch (err) {
        console.error('Failed to cancel task:', err);
    }
};

// Check Empty State
function checkEmptyState() {
    if (activeDownloads.size === 0 && !downloadsContainer.querySelector('.empty-state')) {
        downloadsContainer.innerHTML = '<div class="empty-state">No active downloads</div>';
    }
}

// Reset UI
function resetUI() {
    currentVideoInfo = null;
    titleDisplay.textContent = '';
    titleDisplay.classList.add('hidden');
    resolutionSelect.innerHTML = '<option value="">Select resolution...</option>';
    resolutionSelect.disabled = true;
    addBtn.disabled = true;
    statusLabel.textContent = 'Ready';
}

// Switch Tab
function switchTab(tabName) {
    tabBtns.forEach(btn => {
        btn.classList.toggle('active', btn.dataset.tab === tabName);
    });

    tabContents.forEach(content => {
        content.classList.toggle('active', content.id === tabName);
    });
    
    // Load history when switching to history tab
    if (tabName === 'history') {
        loadHistory();
    }
}

// Toggle Theme
function toggleTheme() {
    isDark = !isDark;
    document.documentElement.setAttribute('data-theme', isDark ? 'dark' : 'light');
    themeIcon.src = isDark ? lightIcon : darkIcon;
}

// Update Output Directory Display
async function updateOutputDir() {
    try {
        const dir = await GetOutputDir();
        if (dir) {
            outputDirDisplay.textContent = `Download Location: ${dir}`;
        } else {
            outputDirDisplay.textContent = 'Download location not set';
        }
    } catch (err) {
        console.error('Failed to get output dir:', err);
        outputDirDisplay.textContent = 'Download location not set';
    }
}

// Extract Resolution
function extractResolution(resolution) {
    const match = resolution.match(/^(\d+)p/);
    if (match) return match[1];
    if (resolution.startsWith('best')) return 'best';
    return '';
}

// Escape HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
