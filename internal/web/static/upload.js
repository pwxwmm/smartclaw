// SmartClaw - Upload
(function() {
  'use strict';

  if (!SC.state.pendingImages) SC.state.pendingImages = [];

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

  function addPendingImage(base64Data, mimeType) {
    if (!SC.state.pendingImages) SC.state.pendingImages = [];
    if (SC.state.pendingImages.length >= 4) {
      SC.toast('Maximum 4 images allowed', 'warn');
      return;
    }
    var sizeBytes = Math.ceil(base64Data.length * 3 / 4);
    if (sizeBytes > 10 * 1024 * 1024) {
      SC.toast('Image exceeds 10MB limit', 'error');
      return;
    }
    SC.state.pendingImages.push({ data: base64Data, type: mimeType, id: 'img-' + Date.now() });
    renderImagePreviews();
  }

  function renderImagePreviews() {
    var container = SC.$('#image-previews');
    if (!container) {
      container = document.createElement('div');
      container.id = 'image-previews';
      var inputArea = SC.$('#input-area');
      if (inputArea) {
        var inputRow = inputArea.querySelector('.input-row');
        if (inputRow) {
          inputArea.insertBefore(container, inputRow);
        }
      }
    }
    container.innerHTML = '';
    if (!SC.state.pendingImages || SC.state.pendingImages.length === 0) return;
    SC.state.pendingImages.forEach(function(img, idx) {
      var item = document.createElement('div');
      item.className = 'img-preview-item';
      var thumb = document.createElement('img');
      thumb.src = 'data:' + img.type + ';base64,' + img.data;
      thumb.className = 'img-preview-thumb';
      var removeBtn = document.createElement('button');
      removeBtn.className = 'img-preview-remove';
      removeBtn.textContent = '\u00d7';
      removeBtn.title = 'Remove';
      removeBtn.addEventListener('click', function() {
        SC.state.pendingImages.splice(idx, 1);
        renderImagePreviews();
      });
      item.appendChild(thumb);
      item.appendChild(removeBtn);
      container.appendChild(item);
    });
  }

  function clearImagePreviews() {
    if (SC.state.pendingImages) SC.state.pendingImages = [];
    var container = SC.$('#image-previews');
    if (container) container.innerHTML = '';
  }

  function initImageHandlers() {
    var input = SC.$('#input');
    if (input) {
      input.addEventListener('paste', function(e) {
        var items = (e.clipboardData || e.originalEvent.clipboardData).items;
        if (!items) return;
        for (var i = 0; i < items.length; i++) {
          if (items[i].type.indexOf('image/') === 0) {
            e.preventDefault();
            var blob = items[i].getAsFile();
            var reader = new FileReader();
            reader.onload = function(ev) {
              var base64 = ev.target.result.split(',')[1];
              addPendingImage(base64, items[i].type);
            };
            reader.readAsDataURL(blob);
            break;
          }
        }
      });
    }

    var chatEl = SC.$('#chat');
    if (chatEl) {
      chatEl.addEventListener('dragover', function(e) {
        if (e.dataTransfer && e.dataTransfer.types) {
          for (var i = 0; i < e.dataTransfer.types.length; i++) {
            if (e.dataTransfer.types[i] === 'Files') {
              var overlay = SC.$('#drag-overlay');
              if (overlay) overlay.classList.remove('hidden');
              break;
            }
          }
        }
      });

      chatEl.addEventListener('drop', function(e) {
        if (e.dataTransfer && e.dataTransfer.files) {
          for (var i = 0; i < e.dataTransfer.files.length; i++) {
            var file = e.dataTransfer.files[i];
            if (file.type.indexOf('image/') === 0) {
              e.preventDefault();
              var reader = new FileReader();
              reader.onload = function(ev) {
                var base64 = ev.target.result.split(',')[1];
                addPendingImage(base64, file.type);
              };
              reader.readAsDataURL(file);
            } else {
              uploadFile(file);
            }
          }
        }
      });
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initImageHandlers);
  } else {
    initImageHandlers();
  }

  SC.uploadFile = uploadFile;
  SC.updateUploadProgress = updateUploadProgress;
  SC.renderUploadProgress = renderUploadProgress;
  SC.addPendingImage = addPendingImage;
  SC.renderImagePreviews = renderImagePreviews;
  SC.clearImagePreviews = clearImagePreviews;
})();
