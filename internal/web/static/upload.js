// SmartClaw - Upload
(function() {
  'use strict';

  function uploadFile(file) {
    const uploadId = 'upl-' + Math.random().toString(36).slice(2, 8);
    SC.state.uploads.push({ id: uploadId, name: file.name, progress: 0, status: 'uploading' });
    renderUploadProgress();

    const formData = new FormData();
    formData.append('file', file);

    const xhr = new XMLHttpRequest();
    xhr.open('POST', '/api/upload');

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) {
        const pct = Math.round((e.loaded / e.total) * 100);
        updateUploadProgress(uploadId, pct);
      }
    };

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          const resp = JSON.parse(xhr.responseText);
          updateUploadProgress(uploadId, 100, 'done');
          SC.toast(`Uploaded ${file.name} (${(resp.size / 1024).toFixed(1)}KB)`, 'success');
          SC.addMessage('user', `Uploaded file: ${resp.path}`);
          setTimeout(() => {
            SC.state.uploads = SC.state.uploads.filter(u => u.id !== uploadId);
            renderUploadProgress();
          }, 2000);
        } catch {
          updateUploadProgress(uploadId, 0, 'error');
          SC.toast(`Upload failed: ${file.name}`, 'error');
        }
      } else {
        let errMsg = 'Upload failed';
        try { errMsg = JSON.parse(xhr.responseText).error || errMsg; } catch {}
        updateUploadProgress(uploadId, 0, 'error');
        SC.toast(errMsg, 'error');
      }
    };

    xhr.onerror = () => {
      updateUploadProgress(uploadId, 0, 'error');
      SC.toast(`Upload failed: ${file.name}`, 'error');
    };

    xhr.send(formData);
  }

  function updateUploadProgress(uploadId, pct, status) {
    const entry = SC.state.uploads.find(u => u.id === uploadId);
    if (entry) {
      entry.progress = pct;
      if (status) entry.status = status;
    }
    renderUploadProgress();
  }

  function renderUploadProgress() {
    let container = SC.$('#upload-overlay');
    const activeUploads = SC.state.uploads.filter(u => u.status !== 'dismissed');

    if (activeUploads.length === 0) {
      if (container) container.remove();
      return;
    }

    if (!container) {
      container = document.createElement('div');
      container.id = 'upload-overlay';
      document.body.appendChild(container);
    }

    container.innerHTML = activeUploads.map(u => {
      const isDone = u.status === 'done';
      const isErr = u.status === 'error';
      return `<div class="upload-item ${isDone ? 'upload-done' : ''} ${isErr ? 'upload-error' : ''}">
        <div class="upload-info">
          <span class="upload-name">${SC.escapeHtml(u.name)}</span>
          <span class="upload-pct">${isDone ? '✓' : isErr ? '✗' : u.progress + '%'}</span>
        </div>
        <div class="upload-bar-track">
          <div class="upload-bar-fill${isDone ? ' upload-bar-done' : isErr ? ' upload-bar-err' : ''}" style="width:${u.progress}%"></div>
        </div>
      </div>`;
    }).join('');
  }

  SC.uploadFile = uploadFile;
  SC.updateUploadProgress = updateUploadProgress;
  SC.renderUploadProgress = renderUploadProgress;
})();
