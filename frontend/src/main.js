// Wails Frontend - Predator YouTube Downloader
import {
    IsValidYouTubeURL,
    IsValidURL,
    FetchVideoInfo,
    FetchPlaylistInfo,
    IsPlaylistURL,
    AddToQueue,
    DownloadPlaylist,
    CancelTask,
    SelectOutputDir,
    GetOutputDir,
    CheckAndInstallDeps,
    GetDownloadHistory,
    OpenFolder,
    ClearHistory,
    ExtractVideoID,
    CheckDuplicate
} from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';

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
const tabBtns = document.querySelectorAll('.tab-btn');
const tabContents = document.querySelectorAll('.tab-content');
const downloadsContainer = document.getElementById('downloads-container');
const historyContainer = document.getElementById('history-container');
const filterBtns = document.querySelectorAll('.filter-btn');
const clearHistoryModal = document.getElementById('clear-history-modal');
const modalCloseBtn = document.getElementById('modal-close-btn');
const modalCancelBtn = document.getElementById('modal-cancel-btn');
const modalConfirmBtn = document.getElementById('modal-confirm-btn');

// Playlist Modal Elements
const playlistModal = document.getElementById('playlist-modal');
const playlistCloseBtn = document.getElementById('playlist-close-btn');
const playlistSelectAllBtn = document.getElementById('playlist-select-all-btn');
const playlistCancelBtn = document.getElementById('playlist-cancel-btn');
const playlistDownloadBtn = document.getElementById('playlist-download-btn');
const playlistItemsContainer = document.getElementById('playlist-items');
const playlistSelectedCount = document.getElementById('playlist-selected-count');

// File Not Found Modal Elements
const fileNotFoundModal = document.getElementById('file-not-found-modal');
const fileNotFoundCloseBtn = document.getElementById('file-not-found-close-btn');
const fileNotFoundOkBtn = document.getElementById('file-not-found-ok-btn');
const fileNotFoundMessage = document.getElementById('file-not-found-message');

// State
let currentVideoInfo = null;
let currentPlaylistInfo = null;
let fetchTimer = null;
let isFetching = false;
let activeDownloads = new Map();
let downloadHistory = [];
let currentFilter = 'all';

// Constants
const FETCH_DEBOUNCE = 600;

// Custom Context Menu for Cut/Copy/Paste
let customContextMenu = null;

function createCustomContextMenu() {
    if (customContextMenu) {
        customContextMenu.remove();
    }

    customContextMenu = document.createElement('div');
    customContextMenu.id = 'custom-context-menu';
    customContextMenu.style.cssText = `
        position: fixed;
        background: var(--bg-card, #1e293b);
        border: 1px solid var(--border-color, #334155);
        border-radius: 6px;
        padding: 4px 0;
        min-width: 120px;
        box-shadow: 0 4px 12px rgba(0,0,0,0.3);
        z-index: 10000;
        display: none;
        font-family: inherit;
        font-size: 13px;
    `;

    const menuItems = [
        { label: 'Cut', action: 'cut', shortcut: 'Ctrl+X' },
        { label: 'Copy', action: 'copy', shortcut: 'Ctrl+C' },
        { label: 'Paste', action: 'paste', shortcut: 'Ctrl+V' },
        { type: 'separator' },
        { label: 'Select All', action: 'selectAll', shortcut: 'Ctrl+A' }
    ];

    menuItems.forEach(item => {
        if (item.type === 'separator') {
            const sep = document.createElement('div');
            sep.style.cssText = 'height: 1px; background: var(--border-color, #334155); margin: 4px 0;';
            customContextMenu.appendChild(sep);
        } else {
            const menuItem = document.createElement('div');
            menuItem.style.cssText = `
                padding: 8px 16px;
                cursor: pointer;
                color: var(--text-primary, #f1f5f9);
                display: flex;
                justify-content: space-between;
                align-items: center;
                gap: 20px;
            `;
            menuItem.innerHTML = `
                <span>${item.label}</span>
                <span style="color: var(--text-muted, #94a3b8); font-size: 11px;">${item.shortcut}</span>
            `;
            menuItem.addEventListener('mouseenter', () => {
                menuItem.style.background = 'var(--bg-hover, #334155)';
            });
            menuItem.addEventListener('mouseleave', () => {
                menuItem.style.background = 'transparent';
            });
            menuItem.addEventListener('click', (e) => {
                e.stopPropagation();
                executeContextMenuAction(item.action);
                hideCustomContextMenu();
            });
            customContextMenu.appendChild(menuItem);
        }
    });

    document.body.appendChild(customContextMenu);

    document.addEventListener('click', hideCustomContextMenu);
    document.addEventListener('scroll', hideCustomContextMenu, true);
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') hideCustomContextMenu();
    });
}

function showCustomContextMenu(e) {
    if (!customContextMenu) createCustomContextMenu();

    const target = e.target;
    if (!target.matches('input, textarea, [contenteditable]')) return;

    e.preventDefault();
    e.stopPropagation();

    const x = Math.min(e.clientX, window.innerWidth - 140);
    const y = Math.min(e.clientY, window.innerHeight - 150);

    customContextMenu.style.left = x + 'px';
    customContextMenu.style.top = y + 'px';
    customContextMenu.style.display = 'block';

    customContextMenu.dataset.targetId = target.id || '';
    customContextMenu._targetElement = target;
}

function hideCustomContextMenu() {
    if (customContextMenu) {
        customContextMenu.style.display = 'none';
    }
}

function executeContextMenuAction(action) {
    const target = customContextMenu?._targetElement || document.activeElement;
    if (!target) return;

    target.focus();

    switch (action) {
        case 'cut':
            if (target.selectionStart !== undefined) {
                const start = target.selectionStart;
                const end = target.selectionEnd;
                const selectedText = target.value.substring(start, end);
                navigator.clipboard.writeText(selectedText).then(() => {
                    target.value = target.value.substring(0, start) + target.value.substring(end);
                    target.setSelectionRange(start, start);
                    target.dispatchEvent(new Event('input', { bubbles: true }));
                });
            }
            break;
        case 'copy':
            if (target.selectionStart !== undefined) {
                const selectedText = target.value.substring(target.selectionStart, target.selectionEnd);
                navigator.clipboard.writeText(selectedText);
            }
            break;
        case 'paste':
            navigator.clipboard.readText().then(text => {
                if (target.selectionStart !== undefined) {
                    const start = target.selectionStart;
                    const end = target.selectionEnd;
                    target.value = target.value.substring(0, start) + text + target.value.substring(end);
                    const newCursorPos = start + text.length;
                    target.setSelectionRange(newCursorPos, newCursorPos);
                    target.dispatchEvent(new Event('input', { bubbles: true }));
                }
            });
            break;
        case 'selectAll':
            if (target.select) {
                target.select();
            }
            break;
    }
}

function setupCustomContextMenu() {
    document.addEventListener('contextmenu', showCustomContextMenu, true);
}

// ---------- Playlist Modal Logic ----------
// Show playlist modal with actual videos from YouTube
function showPlaylistModalWithVideos(playlistInfo) {
    // Update modal title with playlist name
    const playlistTitleEl = document.querySelector('#playlist-modal .modal-title');
    if (playlistTitleEl) {
        playlistTitleEl.textContent = playlistInfo.title || 'Playlist';
    }

    // Populate with actual videos from the playlist
    const videos = playlistInfo.videos || [];
    populatePlaylist(videos);

    // Store video data for download
    window.currentPlaylistVideos = videos;

    // Reset playlist type selector to Video
    const videoRadio = document.querySelector('input[name="playlist-download-type"][value="Video"]');
    if (videoRadio) videoRadio.checked = true;
    const plResWrapper = document.getElementById('playlist-resolution-wrapper');
    const plAudioWrapper = document.getElementById('playlist-audio-format-wrapper');
    if (plResWrapper) plResWrapper.classList.remove('hidden');
    if (plAudioWrapper) plAudioWrapper.classList.add('hidden');

    // Fetch dynamic resolutions from first video in playlist
    const plResSelect = document.getElementById('playlist-resolution-select');
    if (plResSelect && videos.length > 0) {
        // Show loading state
        plResSelect.innerHTML = '<option value="">Loading resolutions...</option>';
        plResSelect.disabled = true;

        // Fetch video info from first video to get available resolutions
        const firstVideoUrl = videos[0].url || `https://youtube.com/watch?v=${videos[0].id}`;
        FetchVideoInfo(firstVideoUrl)
            .then(info => {
                plResSelect.innerHTML = '';
                if (info.resolutions && info.resolutions.length > 0) {
                    info.resolutions.forEach(res => {
                        const option = document.createElement('option');
                        option.value = res;
                        option.textContent = res;
                        plResSelect.appendChild(option);
                    });
                    // Auto-select best resolution (1080p or 720p)
                    const bestIndex = info.resolutions.findIndex(r =>
                        r.includes('1080p') || r.includes('720p')
                    );
                    if (bestIndex >= 0) {
                        plResSelect.selectedIndex = bestIndex;
                    }
                } else {
                    plResSelect.innerHTML = '<option value="best">Best</option>';
                }
                plResSelect.disabled = false;
            })
            .catch(err => {
                console.error('Failed to fetch resolutions for playlist:', err);
                // Fallback to preset options
                plResSelect.innerHTML = `
                    <option value="best" selected>Best</option>
                    <option value="1080">1080p</option>
                    <option value="720">720p</option>
                    <option value="480">480p</option>
                    <option value="360">360p</option>
                `;
                plResSelect.disabled = false;
            });
    }

    // Show the modal
    playlistModal.classList.add('active');
}

function populatePlaylist(videos) {
    // Clear any existing items
    playlistItemsContainer.innerHTML = '';
    videos.forEach((video, idx) => {
        const title = typeof video === 'string' ? video : (video.title || `Video ${idx + 1}`);
        const duration = (typeof video === 'object' && video.duration) ? video.duration : '';

        const wrapper = document.createElement('div');
        wrapper.className = 'playlist-video-item';
        wrapper.innerHTML = `
            <label class="playlist-video-label">
                <input type="checkbox" class="playlist-checkbox playlist-video-checkbox" data-index="${idx}" />
                <span class="playlist-video-number">${idx + 1}</span>
                <span class="playlist-video-title">${escapeHtml(title)}</span>
                ${duration ? `<span class="playlist-video-duration">${duration}</span>` : ''}
            </label>
        `;
        playlistItemsContainer.appendChild(wrapper);
    });

    // Add change listeners to update count
    const checkboxes = playlistItemsContainer.querySelectorAll('.playlist-checkbox');
    checkboxes.forEach(cb => {
        cb.addEventListener('change', updatePlaylistSelectedCount);
    });

    updatePlaylistSelectedCount();
}

function showPlaylistModal() {
    // For now, use a static dummy list of episodes
    const dummyEpisodes = Array.from({ length: 10 }, (_, i) => `Episode ${i + 1}`);
    populatePlaylist(dummyEpisodes);
    playlistModal.classList.add('active');
}

function hidePlaylistModal() {
    playlistModal.classList.remove('active');
}

function toggleSelectAll() {
    const checkboxes = playlistItemsContainer.querySelectorAll('.playlist-checkbox');
    const allSelected = Array.from(checkboxes).every(cb => cb.checked);
    checkboxes.forEach(cb => cb.checked = !allSelected);
    updatePlaylistSelectedCount();
}

function updatePlaylistSelectedCount() {
    const count = playlistItemsContainer.querySelectorAll('.playlist-checkbox:checked').length;
    playlistSelectedCount.textContent = `${count} selected`;
}

// Handle playlist type toggle (Video/Audio) inside the modal
function handlePlaylistTypeChange(e) {
    const type = e.target.value;
    const plResWrapper = document.getElementById('playlist-resolution-wrapper');
    const plAudioWrapper = document.getElementById('playlist-audio-format-wrapper');

    if (type === 'Video') {
        plResWrapper.classList.remove('hidden');
        plAudioWrapper.classList.add('hidden');
    } else {
        plResWrapper.classList.add('hidden');
        plAudioWrapper.classList.remove('hidden');
    }
}

function handlePlaylistDownload() {
    // Get selected video indices
    const selectedCheckboxes = Array.from(playlistItemsContainer.querySelectorAll('.playlist-checkbox:checked'));

    if (selectedCheckboxes.length === 0) {
        statusLabel.textContent = 'No videos selected';
        hidePlaylistModal();
        return;
    }

    // Get selected videos from stored playlist data
    const videos = window.currentPlaylistVideos || [];
    const selectedVideos = selectedCheckboxes.map(cb => {
        const idx = parseInt(cb.dataset.index);
        return videos[idx];
    }).filter(v => v);

    if (selectedVideos.length === 0) {
        statusLabel.textContent = 'No videos selected';
        hidePlaylistModal();
        return;
    }

    // Get download type and format options FROM the playlist modal selectors
    const typeRadio = document.querySelector('input[name="playlist-download-type"]:checked');
    if (!typeRadio) {
        statusLabel.textContent = 'Please select a download type';
        return;
    }
    const type = typeRadio.value;
    const plResSelect = document.getElementById('playlist-resolution-select');
    const plAudioSelect = document.getElementById('playlist-audio-format-select');
    const resolution = type === 'Video' ? plResSelect.value : '';
    const audioFormat = type === 'Audio' ? plAudioSelect.value : 'mp3';

    // Call DownloadPlaylist from backend
    statusLabel.textContent = `Adding ${selectedVideos.length} video(s) to queue...`;

    DownloadPlaylist(selectedVideos, type, resolution, audioFormat)
        .then(taskIds => {
            console.log('Playlist download started, task IDs:', taskIds);
            statusLabel.textContent = `Queued ${selectedVideos.length} video(s)`;

            // Clear the input and switch to history tab
            urlInput.value = '';
            resetUI();
            switchTab('history');
        })
        .catch(err => {
            console.error('Failed to download playlist:', err);
            statusLabel.textContent = 'Failed to add playlist to queue';
        });

    hidePlaylistModal();
}

// ---------- End Playlist Modal Logic ----------

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
    setupCustomContextMenu();

    try {
        await CheckAndInstallDeps();
    } catch (err) {
        console.log('Dependency check:', err);
    }

    updateOutputDir();
    setupEventListeners();
    setupWailsEvents();
});

// Event Listeners
function setupEventListeners() {
    urlInput.addEventListener('input', handleUrlInput);

    clearBtn.addEventListener('click', () => {
        urlInput.value = '';
        resetUI();
    });

    downloadTypeRadios.forEach(radio => {
        radio.addEventListener('change', handleDownloadTypeChange);
    });

    addBtn.addEventListener('click', addToQueue);

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

    tabBtns.forEach(btn => {
        btn.addEventListener('click', () => switchTab(btn.dataset.tab));
    });

    filterBtns.forEach(btn => {
        btn.addEventListener('click', () => setFilter(btn.dataset.filter));
    });

    const clearHistoryBtn = document.getElementById('clear-history-btn');
    if (clearHistoryBtn) {
        clearHistoryBtn.addEventListener('click', showClearHistoryModal);
    }

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

    // File Not Found Modal
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

    // Playlist Modal Listeners (opened automatically when playlist URL is detected)
    if (playlistCloseBtn) {
        playlistCloseBtn.addEventListener('click', hidePlaylistModal);
    }
    if (playlistCancelBtn) {
        playlistCancelBtn.addEventListener('click', hidePlaylistModal);
    }
    if (playlistSelectAllBtn) {
        playlistSelectAllBtn.addEventListener('click', toggleSelectAll);
    }
    if (playlistDownloadBtn) {
        playlistDownloadBtn.addEventListener('click', handlePlaylistDownload);
    }
    // Playlist type toggle (Video/Audio) inside modal
    const playlistTypeRadios = document.querySelectorAll('input[name="playlist-download-type"]');
    playlistTypeRadios.forEach(radio => {
        radio.addEventListener('change', handlePlaylistTypeChange);
    });
    // Close playlist modal on overlay click
    if (playlistModal) {
        playlistModal.addEventListener('click', (e) => {
            if (e.target === playlistModal) {
                hidePlaylistModal();
            }
        });
    }
}

// Wails Event Handlers
function setupWailsEvents() {
    EventsOn('task-started', (taskId, task) => {
        createDownloadTask(taskId, task);
    });

    EventsOn('task-progress', (update) => {
        updateDownloadProgress(update);
    });

    EventsOn('task-retry', (taskId, attempt, maxAttempts) => {
        updateTaskStatus(taskId, `Retrying... (${attempt}/${maxAttempts})`);
    });

    EventsOn('task-completed', (taskId, task) => {
        completeDownloadTask(taskId);
        setTimeout(() => loadHistory(), 1000);
    });

    EventsOn('task-error', (taskId, error) => {
        errorDownloadTask(taskId, error);
    });

    EventsOn('task-cancelled', (taskId) => {
        cancelDownloadTask(taskId);
    });

    EventsOn('installing-deps-progress', (percent, message) => {
        statusLabel.textContent = message ? `${message} ${percent}%` : `Installing... ${percent}%`;
    });

    EventsOn('installing-deps', (installing) => {
        if (installing) {
            statusLabel.textContent = 'Installing dependencies...';
        } else {
            statusLabel.textContent = 'Ready';
        }
    });

}

/**
 * URL Input Handler with Debounce
 * - Supports YouTube/Instagram (existing) and X/Twitter (new)
 * - For X image mode: no need to fetch resolutions; backend picks image formats.
 */
function handleUrlInput() {
    const url = urlInput.value.trim();

    if (fetchTimer) {
        clearTimeout(fetchTimer);
    }

    if (!url) {
        resetUI();
        return;
    }

    fetchTimer = setTimeout(async () => {
        if (isFetching) return;
        isFetching = true;

        try {
            const typeRadio = document.querySelector('input[name="download-type"]:checked');
            const selectedType = typeRadio ? typeRadio.value : 'Video';

            if (!await IsValidURL(url)) {
                statusLabel.textContent = 'Invalid URL';
                titleDisplay.classList.add('hidden');
                resolutionSelect.disabled = true;
                addBtn.disabled = true;
                return;
            }

            let isX = false;
            try {
                const hostname = new URL(url).hostname.toLowerCase();
                isX = hostname === 'x.com' ||
                    hostname.endsWith('.x.com') ||
                    hostname === 'twitter.com' ||
                    hostname.endsWith('.twitter.com');
            } catch {
                isX = false;
            }

            // X image: backend handles image format selection, don't fetch video info
            if (selectedType === 'Image' && isX) {
                currentVideoInfo = null;
                titleDisplay.textContent = 'X Images';
                titleDisplay.classList.remove('hidden');

                resolutionSelect.disabled = true;
                addBtn.disabled = false;

                resolutionWrapper.classList.add('hidden');
                audioFormatWrapper.classList.add('hidden');
                return;
            }

            // Check if URL is a playlist
            const isPlaylist = await IsPlaylistURL(url);

            if (isPlaylist) {
                // Playlist UI currently assumes Video/Audio
                statusLabel.textContent = 'Fetching playlist info...';
                resolutionSelect.disabled = true;
                addBtn.disabled = true;

                try {
                    const playlistInfo = await FetchPlaylistInfo(url);
                    currentPlaylistInfo = playlistInfo;

                    titleDisplay.textContent = `Playlist: ${playlistInfo.title} (${playlistInfo.videoCount} videos)`;
                    titleDisplay.classList.remove('hidden');

                    showPlaylistModalWithVideos(playlistInfo);
                    statusLabel.textContent = 'Select videos to download';
                } catch (err) {
                    console.error('Fetch playlist error:', err);
                    statusLabel.textContent = 'Failed to fetch playlist';
                    titleDisplay.classList.add('hidden');
                }
            } else {
                // Single item
                statusLabel.textContent = 'Fetching video info...';
                resolutionSelect.disabled = true;
                addBtn.disabled = true;

                // For Image mode (non-X or if frontend can't infer), keep UX safe: allow queue without resolutions/audio.
                if (selectedType === 'Image') {
                    currentVideoInfo = null;
                    titleDisplay.textContent = 'Image Download';
                    titleDisplay.classList.remove('hidden');
                    resolutionWrapper.classList.add('hidden');
                    audioFormatWrapper.classList.add('hidden');
                    addBtn.disabled = false;
                    return;
                }

                try {
                    const info = await FetchVideoInfo(url);
                    currentVideoInfo = info;
                    displayVideoInfo(info);
                    statusLabel.textContent = 'Ready to download';
                } catch (err) {
                    console.error('Fetch error:', err);
                    statusLabel.textContent = 'Failed to fetch info';
                    titleDisplay.classList.add('hidden');
                }
            }
        } finally {
            isFetching = false;
        }
    }, FETCH_DEBOUNCE);
}

// Display Video Info
function displayVideoInfo(info) {
    titleDisplay.textContent = `Title: ${info.title}`;
    titleDisplay.classList.remove('hidden');

    resolutionSelect.innerHTML = '';
    info.resolutions.forEach(res => {
        const option = document.createElement('option');
        option.value = res;
        option.textContent = res;
        resolutionSelect.appendChild(option);
    });

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
        addBtn.disabled = !currentVideoInfo;
    } else if (type === 'Audio') {
        resolutionWrapper.classList.add('hidden');
        audioFormatWrapper.classList.remove('hidden');
        resolutionSelect.disabled = true;
        addBtn.disabled = false;
    } else if (type === 'Image') {
        // For X images: always disable resolution/audio
        resolutionWrapper.classList.add('hidden');
        audioFormatWrapper.classList.add('hidden');
        resolutionSelect.disabled = true;
        addBtn.disabled = !urlInput.value.trim();
    }
}

// Add to Queue - handles both single video and playlist
async function addToQueue() {
    const url = urlInput.value.trim();
    const typeRadio = document.querySelector('input[name="download-type"]:checked');
    if (!typeRadio) {
        statusLabel.textContent = 'Please select a download type';
        return;
    }
    const type = typeRadio.value;

    if (!url || !await IsValidURL(url)) {
        statusLabel.textContent = 'Please enter a valid URL';
        return;
    }

    // Disable button immediately to prevent double click
    addBtn.disabled = true;

    // Check if URL is a playlist
    const isPlaylist = await IsPlaylistURL(url);

    if (isPlaylist && currentPlaylistInfo) {
        // If playlist is detected, show the playlist modal
        showPlaylistModalWithVideos(currentPlaylistInfo);
        statusLabel.textContent = 'Select videos to download';
        addBtn.disabled = false;
        return;
    }

    // Check for duplicate video ID before queueing
    try {
        const videoID = await ExtractVideoID(url);
        if (videoID) {
            const dupCheck = await CheckDuplicate(videoID);
            if (dupCheck && dupCheck.isDuplicate) {
                const existingFile = dupCheck.existingItem.filePath;
                const baseName = existingFile ? existingFile.split(/[\\/]/).pop() : 'the file';
                const confirmDownload = confirm(`This media has already been downloaded.\nFile: ${baseName}\n\nDo you want to download it again?`);
                if (!confirmDownload) {
                    resetUI();
                    return;
                }
            }
        }
    } catch (err) {
        console.error('Duplicate check failed:', err);
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

    // X images: resolution/audio should be omitted; backend ignores/doesn't need them
    if (type === 'Image') {
        task.resolution = '';
        task.cleanRes = '';
        task.audioFormat = '';
    }

    try {
        const taskId = await AddToQueue(task);
        console.log('Added to queue:', taskId);

        urlInput.value = '';
        resetUI();

        switchTab('history');

        statusLabel.textContent = 'Added to queue';
    } catch (err) {
        console.error('Failed to add to queue:', err);
        statusLabel.textContent = 'Failed to add to queue';
        addBtn.disabled = false;
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

    filterBtns.forEach(btn => {
        btn.classList.toggle('active', btn.dataset.filter === filter);
    });

    renderHistory();
}

// Render History
function renderHistory() {
    let filtered = downloadHistory;
    if (currentFilter !== 'all') {
        filtered = downloadHistory.filter(item => item.type === currentFilter);
    }

    historyContainer.innerHTML = '';

    if (filtered.length === 0) {
        historyContainer.innerHTML = '<div class="empty-state">No download history</div>';
        return;
    }

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

    const rawFilePath = item.filePath || '';
    const encodedPath = encodeURIComponent(rawFilePath);

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
        <button class="history-item-folder-btn" title="Open folder" data-filepath="${encodedPath}">
            📁
        </button>
    `;

    const folderBtn = div.querySelector('.history-item-folder-btn');
    folderBtn.addEventListener('click', async () => {
        const path = folderBtn.getAttribute('data-filepath');
        if (path) {
            try {
                const decodedPath = decodeURIComponent(path);
                console.log('Opening folder:', decodedPath);
                await OpenFolder(decodedPath);
            } catch (err) {
                console.error('Failed to open folder:', err);
                showFileNotFoundModal('The file or folder could not be found. It may have been moved or deleted.');
            }
        }
    });

    return div;
}

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

    setTimeout(() => {
        taskElement.remove();
        activeDownloads.delete(taskId);
        checkEmptyState();
    }, 2000);
}

// Cancel Task (called from UI)
window.cancelTask = async function (taskId) {
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

    if (tabName === 'history') {
        loadHistory();
    }
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
