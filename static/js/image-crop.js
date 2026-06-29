// Shared image upload + crop flow — used by the avatar picker on the
// classic dashboard and by image blocks in the canvas builder. Loads
// Cropper.js lazily (only when first opened) so pages that never touch
// images don't pay for it.
(function () {
    var modal, imgEl, cropper, currentOpts;
    var cropperAssetsLoaded = false;

    function ensureModal() {
        if (modal) return;
        modal = document.createElement('div');
        modal.id = 'foliocv-crop-modal';
        modal.className = 'fixed inset-0 z-[100] hidden items-center justify-center bg-black/70 backdrop-blur-sm';
        modal.innerHTML =
            '<div class="glass rounded-2xl p-5 w-full max-w-lg mx-4">' +
            '  <h3 class="font-display font-semibold mb-3">Crop image</h3>' +
            '  <div class="bg-black/40 rounded-lg overflow-hidden" style="max-height: 60vh;">' +
            '    <img id="foliocv-crop-img" style="max-width: 100%;">' +
            '  </div>' +
            '  <div class="flex items-center justify-end gap-2 mt-4">' +
            '    <button type="button" id="foliocv-crop-cancel" class="text-sm text-gray-400 hover:text-gray-200 px-3 py-2">Cancel</button>' +
            '    <button type="button" id="foliocv-crop-save" class="text-sm btn-primary rounded-lg px-4 py-2">Save</button>' +
            '  </div>' +
            '</div>';
        document.body.appendChild(modal);
        imgEl = document.getElementById('foliocv-crop-img');

        document.getElementById('foliocv-crop-cancel').addEventListener('click', closeModal);
        document.getElementById('foliocv-crop-save').addEventListener('click', saveCrop);
    }

    function loadCropperAssets(cb) {
        if (cropperAssetsLoaded || window.Cropper) {
            cropperAssetsLoaded = true;
            cb();
            return;
        }
        var link = document.createElement('link');
        link.rel = 'stylesheet';
        link.href = 'https://cdn.jsdelivr.net/npm/cropperjs@1/dist/cropper.min.css';
        document.head.appendChild(link);

        var script = document.createElement('script');
        script.src = 'https://cdn.jsdelivr.net/npm/cropperjs@1/dist/cropper.min.js';
        script.onload = function () {
            cropperAssetsLoaded = true;
            cb();
        };
        document.head.appendChild(script);
    }

    function openModal(dataUrl, aspectRatio) {
        ensureModal();
        modal.classList.remove('hidden');
        modal.classList.add('flex');
        imgEl.src = dataUrl;

        loadCropperAssets(function () {
            if (cropper) cropper.destroy();
            cropper = new Cropper(imgEl, {
                aspectRatio: aspectRatio || NaN,
                viewMode: 1,
                autoCropArea: 1,
            });
        });
    }

    function closeModal() {
        if (cropper) {
            cropper.destroy();
            cropper = null;
        }
        if (modal) {
            modal.classList.add('hidden');
            modal.classList.remove('flex');
        }
        currentOpts = null;
    }

    function saveCrop() {
        if (!cropper || !currentOpts) return;
        cropper.getCroppedCanvas().toBlob(function (blob) {
            var formData = new FormData();
            formData.append('image', blob, 'crop.png');
            formData.append('kind', currentOpts.kind || 'image');

            fetch('/dashboard/images', { method: 'POST', body: formData })
                .then(function (res) {
                    if (!res.ok) throw new Error('upload failed');
                    return res.json();
                })
                .then(function (data) {
                    closeModal();
                    if (currentOpts && currentOpts.onSuccess) currentOpts.onSuccess(data);
                })
                .catch(function () {
                    alert('Image upload failed. Please try again.');
                });
        }, 'image/png');
    }

    // FolioCVCropper.open({ aspectRatio, kind, onSuccess(data) })
    // Opens a file picker, then the crop modal, then uploads on save.
    window.FolioCVCropper = {
        open: function (opts) {
            currentOpts = opts || {};
            var input = document.createElement('input');
            input.type = 'file';
            input.accept = 'image/jpeg,image/png';
            input.addEventListener('change', function () {
                var file = input.files && input.files[0];
                if (!file) return;
                var reader = new FileReader();
                reader.onload = function (e) {
                    openModal(e.target.result, currentOpts.aspectRatio);
                };
                reader.readAsDataURL(file);
            });
            input.click();
        }
    };
})();
