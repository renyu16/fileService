const API_BASE = '/api';
let currentRole = '';

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
        xhr.open('POST', API_BASE + '/files');

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
                loadFiles();
            } else {
                try {
                    const resp = JSON.parse(xhr.responseText);
                    showMessage(resp.error || '上传失败', 'error');
                } catch (err) {
                    showMessage('上传失败', 'error');
                }
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

    refreshBtn.addEventListener('click', loadFiles);

    initUser();
    loadFiles();
});

async function initUser() {
    const loginStatus = document.getElementById('loginStatus');
    const uploadSection = document.getElementById('uploadSection');
    try {
        const resp = await fetch(API_BASE + '/auth/me', { credentials: 'same-origin' });
        if (resp.ok) {
            const data = await resp.json();
            currentRole = data.user.role;
            const isAdmin = currentRole === 'admin';
            loginStatus.innerHTML = `
                <span>当前用户：${escapeHtml(data.user.username)}${isAdmin ? '（管理员）' : ''}</span>
                <a href="/static/change-password.html" class="btn btn-small">修改密码</a>
                ${isAdmin ? '<a href="/static/admin.html" class="btn btn-small">账号管理</a>' : ''}
                <button onclick="logout()" class="btn btn-small btn-secondary">退出登录</button>
            `;
            uploadSection.classList.remove('hidden');
        } else {
            loginStatus.innerHTML = '<a href="/static/login.html" class="btn btn-small">登录</a>';
            uploadSection.classList.add('hidden');
        }
    } catch (err) {
        loginStatus.innerHTML = '<a href="/static/login.html" class="btn btn-small">登录</a>';
        uploadSection.classList.add('hidden');
    }
}

async function logout() {
    try {
        await fetch(API_BASE + '/auth/logout', { method: 'POST', credentials: 'same-origin' });
        window.location.reload();
    } catch (err) {
        alert('退出失败');
    }
}

async function loadFiles() {
    const filesList = document.getElementById('filesList');
    filesList.innerHTML = '<p class="loading">加载中...</p>';

    try {
        const resp = await fetch(API_BASE + '/files');
        const data = await resp.json();
        const files = data.files || [];

        if (files.length === 0) {
            filesList.innerHTML = '<p class="empty">暂无文件</p>';
            return;
        }

        filesList.innerHTML = files.map(file => {
            const isLoggedIn = currentRole !== '';
            const renameBtn = isLoggedIn
                ? `<button onclick="renameFile('${escapeHtml(file.name)}', '${escapeHtml(file.display_name || '')}')" class="btn btn-small">改名</button>`
                : '';
            const deleteBtn = currentRole === 'admin'
                ? `<button onclick="deleteFile('${escapeHtml(file.name)}')" class="btn btn-delete">删除</button>`
                : '';
            const displayName = file.display_name ? escapeHtml(file.display_name) : escapeHtml(file.name);
            return `
                <div class="file-item">
                    <div class="file-info">
                        <div class="file-name">${displayName}</div>
                        <div class="file-meta">${escapeHtml(file.name)} | ${file.size_human} | ${formatDate(file.modified)}</div>
                    </div>
                    <div class="file-actions">
                        <a href="${file.download_url}" class="btn btn-download">下载</a>
                        ${renameBtn}
                        ${deleteBtn}
                    </div>
                </div>
            `;
        }).join('');
    } catch (err) {
        filesList.innerHTML = '<p class="empty">加载失败</p>';
    }
}

async function renameFile(filename, currentName) {
    const newName = prompt('请输入新的名称（留空则恢复为文件名）：', currentName || '');
    if (newName === null) return;

    try {
        const resp = await fetch(`${API_BASE}/files/${encodeURIComponent(filename)}/name`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'same-origin',
            body: JSON.stringify({ name: newName })
        });
        const data = await resp.json();
        if (resp.ok) {
            loadFiles();
        } else {
            showMessage(data.error || '改名失败', 'error');
        }
    } catch (err) {
        showMessage('网络错误', 'error');
    }
}

async function deleteFile(filename) {
    if (!confirm('确认删除 ' + filename + '?')) return;
    try {
        const resp = await fetch(`${API_BASE}/files/${encodeURIComponent(filename)}`, { method: 'DELETE', credentials: 'same-origin' });
        if (resp.ok) {
            loadFiles();
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