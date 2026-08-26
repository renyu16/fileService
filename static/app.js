const API_BASE = '/api';

document.addEventListener('DOMContentLoaded', () => {
    const uploadForm = document.getElementById('uploadForm');
    const fileInput = document.getElementById('fileInput');
    const fileName = document.getElementById('fileName');
    const uploadBtn = document.getElementById('uploadBtn');
    const uploadProgress = document.getElementById('uploadProgress');
    const progressFill = document.getElementById('progressFill');
    const progressText = document.getElementById('progressText');
    const uploadMessage = document.getElementById('uploadMessage');
    const refreshBtn = document.getElementById('refreshBtn');

    fileInput.addEventListener('change', () => {
        fileName.textContent = fileInput.files.length > 0 ? fileInput.files[0].name : '未选择文件';
    });

    uploadForm.addEventListener('submit', (e) => {
        e.preventDefault();
        const file = fileInput.files[0];
        if (!file) {
            showMessage('请选择文件', 'error');
            return;
        }

        const formData = new FormData();
        formData.append('file', file);

        const xhr = new XMLHttpRequest();
        xhr.open('POST', API_BASE + '/apks');

        xhr.upload.onprogress = (e) => {
            if (e.lengthComputable) {
                const percent = Math.round((e.loaded / e.total) * 100);
                progressFill.style.width = percent + '%';
                progressText.textContent = percent + '%';
            }
        };

        xhr.onload = () => {
            uploadProgress.classList.add('hidden');
            if (xhr.status === 200) {
                showMessage('上传成功!', 'success');
                fileInput.value = '';
                fileName.textContent = '未选择文件';
                loadAPKs();
            } else {
                const resp = JSON.parse(xhr.responseText);
                showMessage(resp.error || '上传失败', 'error');
            }
        };

        xhr.onerror = () => {
            uploadProgress.classList.add('hidden');
            showMessage('网络错误', 'error');
        };

        uploadProgress.classList.remove('hidden');
        progressFill.style.width = '0%';
        progressText.textContent = '0%';
        xhr.send(formData);
    });

    refreshBtn.addEventListener('click', loadAPKs);

    loadAPKs();
});

async function loadAPKs() {
    const filesList = document.getElementById('filesList');
    filesList.innerHTML = '<p class="loading">加载中...</p>';

    try {
        const resp = await fetch(API_BASE + '/apks');
        const data = await resp.json();
        const apks = data.apks || [];

        if (apks.length === 0) {
            filesList.innerHTML = '<p class="empty">暂无文件</p>';
            return;
        }

        filesList.innerHTML = apks.map(apk => `
            <div class="file-item">
                <div class="file-info">
                    <div class="file-name">${escapeHtml(apk.name)}</div>
                    <div class="file-meta">${apk.size_human} | ${formatDate(apk.modified)}</div>
                </div>
                <div class="file-actions">
                    <a href="${apk.download_url}" class="btn btn-download">下载</a>
                    <button onclick="deleteAPK('${escapeHtml(apk.name)}')" class="btn btn-delete">删除</button>
                </div>
            </div>
        `).join('');
    } catch (err) {
        filesList.innerHTML = '<p class="empty">加载失败</p>';
    }
}

async function deleteAPK(filename) {
    if (!confirm('确认删除 ' + filename + '?')) return;
    try {
        const resp = await fetch(`${API_BASE}/apks/${encodeURIComponent(filename)}`, { method: 'DELETE' });
        if (resp.ok) {
            loadAPKs();
        } else {
            const data = await resp.json();
            showMessage(data.error || '删除失败', 'error');
        }
    } catch (err) {
        showMessage('网络错误', 'error');
    }
}

function showMessage(text, type) {
    const msg = document.getElementById('uploadMessage');
    msg.textContent = text;
    msg.className = 'message ' + type;
    setTimeout(() => { msg.className = 'message'; }, 3000);
}

function formatDate(iso) {
    const d = new Date(iso);
    return d.toLocaleString('zh-CN');
}

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}