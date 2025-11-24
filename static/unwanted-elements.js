// Unwanted Elements Management Page JavaScript

let uploadedPdfFile = null; // Store the uploaded File object
let currentAnalysis = null; // Store current analysis results

document.addEventListener('DOMContentLoaded', function() {
    // Elements
    const uploadForm = document.getElementById('unwantedElementsUploadForm');
    const analyzeBtn = document.getElementById('analyzeBtn');
    const progressIndicator = document.getElementById('progressIndicator');
    const analysisResults = document.getElementById('analysisResults');
    const analysisSummary = document.getElementById('analysisSummary');
    const unwantedElementsGrid = document.getElementById('unwantedElementsGrid');
    const selectionControls = document.getElementById('selectionControls');
    const selectAllBtn = document.getElementById('selectAllBtn');
    const clearSelectionBtn = document.getElementById('clearSelectionBtn');
    const removeSelectedBtn = document.getElementById('removeSelectedBtn');
    const result = document.getElementById('result');
    const resultContent = document.getElementById('resultContent');

    // File upload handling
    uploadForm.addEventListener('change', function(e) {
        const file = e.target.files[0];
        if (file && file.type === 'application/pdf') {
            uploadedPdfFile = file;
            analyzeBtn.disabled = false;
            analyzeBtn.style.backgroundColor = '#4caf50';
        } else {
            uploadedPdfFile = null;
            analyzeBtn.disabled = true;
            analyzeBtn.style.backgroundColor = '#ccc';
        }
        });
    }
    }

    // Analyze button click
    analyzeBtn.addEventListener('click', async function() {
        if (!uploadedPdfFile) {
            showResult('Please select a PDF file first.', 'error');
            return;
        }

        try {
            // Show progress
            progressIndicator.style.display = 'block';
            analyzeBtn.disabled = true;
            analyzeBtn.textContent = 'Analyzing...';

            // Prepare form data
            const formData = new FormData();
            formData.append('pdf', uploadedPdfFile);

            // Call analysis API
            const response = await fetch('/api/pdf/analyze-unwanted-elements', {
                method: 'POST',
                body: formData
            });

            if (response.ok) {
                currentAnalysis = await response.json();
                displayAnalysisResults(currentAnalysis);
                showResult('Analysis complete! Review the detected unwanted elements below.', 'success');
            } else {
                const errorText = await response.text();
                throw new Error(errorText || 'Analysis failed');
            }
        } catch (error) {
            console.error('Analysis error:', error);
            showResult('Failed to analyze PDF. Please try again.', 'error');
        } finally {
            progressIndicator.style.display = 'none';
            analyzeBtn.disabled = false;
            analyzeBtn.textContent = 'Start Analysis';
        }
    });

    // Select all button
    selectAllBtn.addEventListener('click', function() {
        document.querySelectorAll('.unwanted-element-checkbox').forEach(cb => {
            cb.checked = true;
        });
        updateSelectedCount();
    });

    // Clear selection button
    clearSelectionBtn.addEventListener('click', function() {
        document.querySelectorAll('.unwanted-element-checkbox').forEach(cb => {
            cb.checked = false;
        });
        updateSelectedCount();
    });

    // Remove selected button
    removeSelectedBtn.addEventListener('click', async function() {
        const selectedIds = [];
        document.querySelectorAll('.unwanted-element-checkbox:checked').forEach(cb => {
            selectedIds.push(cb.value);
        });

        if (selectedIds.length === 0) {
            showResult('Please select at least one unwanted element to remove.', 'error');
            return;
        }

        try {
            showResult('Removing selected unwanted elements...', 'loading');
            removeSelectedBtn.disabled = true;
            removeSelectedBtn.textContent = 'Processing...';

            // Prepare form data
            const formData = new FormData();
            formData.append('pdf', uploadedPdfFile);
            formData.append('elements', selectedIds.join(','));

            // Call removal API (this would need to be implemented)
            const response = await fetch('/api/pdf/remove-selected-elements', {
                method: 'POST',
                body: formData
            });

            if (response.ok) {
                // Create download link
                const blob = await response.blob();
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.style.display = 'none';
                a.href = url;
                a.download = 'unwanted_elements_removed.pdf';
                document.body.appendChild(a);
                a.click();
                window.URL.revokeObjectURL(url);
                a.remove();

                showResult('Unwanted elements removed successfully! Download should start automatically.', 'success');
            } else {
                const errorText = await response.text();
                throw new Error(errorText || 'Removal failed');
            }
        } catch (error) {
            console.error('Removal error:', error);
            showResult('Failed to remove unwanted elements. Please try again.', 'error');
        } finally {
            removeSelectedBtn.disabled = false;
            removeSelectedBtn.textContent = 'Remove Selected Unwanted Elements';
        }
    });

    // Display analysis results
    function displayAnalysisResults(analysis) {
        // Show summary
        analysisSummary.innerHTML = `
            <div style="background: #e3f2fd; padding: 10px; border-radius: 4px; margin-bottom: 15px;">
                <strong>Summary:</strong>
                <ul style="margin-top: 5px;">
                    <li>Total Pages: ${analysis.total_pages}</li>
                    <li>Potential Unwanted Elements Found: ${analysis.image_candidates.length + analysis.text_candidates.length}</li>
                    <li>Overall Confidence: ${(analysis.overall_confidence * 100).toFixed(1)}%</li>
                </ul>
            </div>
            ${analysis.debug_logs && analysis.debug_logs.length > 0 ? `
            <div style="background: #fff3e0; padding: 10px; border-radius: 4px; margin-bottom: 15px;">
                <strong>Debug Information:</strong>
                <button id="toggleDebugBtn" style="margin: 5px 0; padding: 5px 10px; cursor: pointer;">Show Debug Logs</button>
                <div id="debugLogsContent" style="display: none; max-height: 400px; overflow-y: auto; background: #f5f5f5; padding: 10px; border-radius: 4px; font-family: monospace; font-size: 11px; white-space: pre-wrap; word-wrap: break-word; margin-top: 10px;">
                    ${analysis.debug_logs.join('\n')}
                </div>
            </div>
            ` : ''}
        `;

        // Add toggle for debug logs if they exist
        if (analysis.debug_logs && analysis.debug_logs.length > 0) {
            const toggleBtn = document.getElementById('toggleDebugBtn');
            if (toggleBtn) {
                toggleBtn.addEventListener('click', function() {
                    const debugContent = document.getElementById('debugLogsContent');
                    if (debugContent.style.display === 'none') {
                        debugContent.style.display = 'block';
                        toggleBtn.textContent = 'Hide Debug Logs';
                    } else {
                        debugContent.style.display = 'none';
                        toggleBtn.textContent = 'Show Debug Logs';
                    }
                });
            }
        }

        // Display recommendations
        if (analysis.recommendations && analysis.recommendations.length > 0) {
            analysisSummary.innerHTML += `
                <div style="background: #fff3e0; padding: 10px; border-radius: 4px;">
                    <strong>Recommendations:</strong>
                    <ul style="margin-top: 5px;">
                        ${analysis.recommendations.map(rec => `<li>${rec}</li>`).join('')}
                    </ul>
                </div>
            `;
        }

        // Clear previous results
        unwantedElementsGrid.innerHTML = '';

        // Display unwanted element candidates
        const allCandidates = [...analysis.image_candidates, ...analysis.text_candidates];

        if (allCandidates.length === 0) {
            unwantedElementsGrid.innerHTML = '<p style="text-align: center; color: #666;">No potential unwanted elements detected in this PDF.</p>';
            return;
        }

        // Show selection controls
        selectionControls.style.display = 'block';

        allCandidates.forEach(candidate => {
            const itemDiv = createUnwantedElementItem(candidate, analysis);
            unwantedElementsGrid.appendChild(itemDiv);
        });

        analysisResults.style.display = 'block';
        updateSelectedCount();
    }

    // Create unwanted element item element
    function createUnwantedElementItem(candidate, analysis) {
        const itemDiv = document.createElement('div');
        itemDiv.className = 'unwanted-element-item';
        itemDiv.setAttribute('data-id', candidate.id);

        // Header with type and confidence
        const header = document.createElement('div');
        header.className = 'unwanted-element-header';

        const typeSpan = document.createElement('span');
        typeSpan.className = `candidate-type ${candidate.type}`;
        typeSpan.textContent = candidate.type.toUpperCase();

        const confidenceSpan = document.createElement('span');
        confidenceSpan.className = `confidence-score ${getConfidenceClass(candidate.confidence)}`;
        confidenceSpan.textContent = `${(candidate.confidence * 100).toFixed(1)}% confidence`;

        header.appendChild(typeSpan);
        header.appendChild(confidenceSpan);

        // Description
        const description = document.createElement('div');
        description.innerHTML = `<strong>Description:</strong><br>${candidate.description}`;

        // Preview - will be loaded asynchronously
        const preview = document.createElement('div');
        preview.className = 'unwanted-element-preview';
        preview.innerHTML = '<div style="text-align: center; color: #666; padding: 20px;">Loading preview...</div>';
        
        // Load preview if we have the PDF file ID
        if (candidate.type === 'image' && analysis && analysis.pdf_file_id) {
            loadPreview(candidate, analysis.pdf_file_id, preview);
        } else {
            preview.innerHTML = '<div style="text-align: center; color: #999; padding: 20px;">Preview unavailable</div>';
        }

        // Checkbox
        const checkboxContainer = document.createElement('div');
        checkboxContainer.className = 'checkbox-container';

        const checkbox = document.createElement('input');
        checkbox.type = 'checkbox';
        checkbox.className = 'unwanted-element-checkbox';
        checkbox.value = candidate.id;
        checkbox.id = `wm-${candidate.id}`;

        const label = document.createElement('label');
        label.htmlFor = `wm-${candidate.id}`;
        label.textContent = 'Select for removal';

        checkboxContainer.appendChild(checkbox);
        checkboxContainer.appendChild(label);

        // Metadata
        if (candidate.metadata) {
            const metadata = document.createElement('div');
            metadata.style.fontSize = '0.8em';
            metadata.style.color = '#666';
            metadata.style.marginTop = '5px';
            metadata.innerHTML = '<strong>Details:</strong> ' +
                Object.entries(candidate.metadata)
                    .filter(([key]) => key !== 'type') // Filter out type since it's already shown
                    .map(([key, value]) => `${key}: ${value}`)
                    .join(', ');
            itemDiv.appendChild(metadata);
        }

        itemDiv.appendChild(header);
        itemDiv.appendChild(description);
        itemDiv.appendChild(preview);
        itemDiv.appendChild(checkboxContainer);

        // Checkbox change handler
        checkbox.addEventListener('change', updateSelectedCount);

        return itemDiv;
    }

    // Update selected count display
    function updateSelectedCount() {
        const selectedCount = document.querySelectorAll('.unwanted-element-checkbox:checked').length;
        const totalCount = document.querySelectorAll('.unwanted-element-checkbox').length;

        if (selectedCount > 0) {
            removeSelectedBtn.textContent = `Remove Selected Unwanted Elements (${selectedCount})`;
            removeSelectedBtn.disabled = false;
        } else {
            removeSelectedBtn.textContent = 'Remove Selected Unwanted Elements';
            removeSelectedBtn.disabled = true;
        }
    }

    // Load preview image for a candidate
    async function loadPreview(candidate, pdfFileId, previewContainer) {
        try {
            const previewUrl = `/api/pdf/preview-image?pdf_file_id=${encodeURIComponent(pdfFileId)}&element_id=${encodeURIComponent(candidate.id)}`;
            
            // Create img element
            const img = document.createElement('img');
            img.style.maxWidth = '100%';
            img.style.maxHeight = '200px';
            img.style.objectFit = 'contain';
            img.style.borderRadius = '4px';
            
            img.onload = function() {
                previewContainer.innerHTML = '';
                previewContainer.appendChild(img);
            };
            
            img.onerror = function() {
                previewContainer.innerHTML = '<div style="text-align: center; color: #999; padding: 20px;">Preview unavailable</div>';
            };
            
            img.src = previewUrl;
        } catch (error) {
            console.error('Failed to load preview:', error);
            previewContainer.innerHTML = '<div style="text-align: center; color: #999; padding: 20px;">Preview unavailable</div>';
        }
    }

    // Helper functions
    function getConfidenceClass(confidence) {
        if (confidence >= 0.8) return 'high';
        if (confidence >= 0.5) return 'medium';
        return 'low';
    }

    function showResult(message, type) {
        resultContent.className = type;
        resultContent.textContent = message;
        result.style.display = 'block';

        if (type === 'success' || type === 'error') {
            setTimeout(() => {
                result.style.display = 'none';
            }, 6000);
        }
    }
});
