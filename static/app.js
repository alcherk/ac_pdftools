// PDF Editor JavaScript

let uploadedPdfFile = null; // Store the uploaded File object

document.addEventListener('DOMContentLoaded', function() {
    // Upload form handler
    const uploadForm = document.getElementById('uploadForm');
    const operationsDiv = document.getElementById('operations');
    const resultDiv = document.getElementById('result');
    const resultContent = document.getElementById('resultContent');

    uploadForm.addEventListener('submit', async function(e) {
        e.preventDefault();

        const formData = new FormData(this);
        const file = formData.get('pdf');

        if (!file || file.size === 0) {
            showResult('Please select a PDF file first.', 'error');
            return;
        }

        try {
            showResult('Uploading PDF...', 'loading');

            const uploadResponse = await fetch('/api/pdf/upload', {
                method: 'POST',
                body: formData
            });

            if (uploadResponse.ok) {
                const uploadResult = await uploadResponse.json();
                uploadedPdfFile = file; // Store the file object for operations
                showResult('PDF uploaded successfully! You can now perform operations.', 'success');
                operationsDiv.style.display = 'block';
                resultDiv.style.display = 'none';
            } else {
                throw new Error('Upload failed');
            }
        } catch (error) {
            console.error('Upload error:', error);
            showResult('Failed to upload PDF. Please try again.', 'error');
        }
    });

    // Handle operation forms
    document.querySelectorAll('.operation-form').forEach(form => {
        form.addEventListener('submit', async function(e) {
            e.preventDefault();
            const endpoint = this.getAttribute('data-endpoint');

            if (!uploadedPdfFile) {
                showResult('No PDF file uploaded. Please upload a file first.', 'error');
                return;
            }

            try {
                showResult('Processing PDF...', 'loading');

                const formData = new FormData(this);
                formData.append('pdf', uploadedPdfFile);

                const response = await fetch(endpoint, {
                    method: 'POST',
                    body: formData
                });

                if (response.ok) {
                    // Create download link for the processed file
                    const blob = await response.blob();
                    const url = window.URL.createObjectURL(blob);
                    const a = document.createElement('a');
                    a.style.display = 'none';
                    a.href = url;

                    // Get filename from response headers
                    const contentDisposition = response.headers.get('Content-Disposition');
                    let filename = 'processed.pdf';
                    if (contentDisposition) {
                        const match = contentDisposition.match(/filename="(.+)"/);
                        if (match) {
                            filename = match[1];
                        }
                    }

                    a.download = filename;
                    document.body.appendChild(a);
                    a.click();
                    window.URL.revokeObjectURL(url);
                    a.remove();

                    showResult('PDF processed successfully! Download should start automatically.', 'success');
                } else {
                    const errorText = await response.text();
                    throw new Error(errorText || 'Processing failed');
                }
            } catch (error) {
                console.error('Processing error:', error);
                let errorMessage = 'Failed to process PDF. ';
                if (error.message.includes('pages')) {
                    errorMessage += 'Please check your page specification format.';
                } else if (error.message.includes('elements')) {
                    errorMessage += 'Element removal is not yet fully implemented.';
                } else {
                    errorMessage += 'Please try again.';
                }
                showResult(errorMessage, 'error');
            }
        });
    });

    // Handle analyze unwanted elements button
    const analyzeBtn = document.getElementById('analyzeBtn');
    const analysisResults = document.getElementById('analysisResults');
    const analysisContent = document.getElementById('analysisContent');
    const elementSelection = document.getElementById('elementSelection');
    const elementCheckboxes = document.getElementById('elementCheckboxes');

    analyzeBtn.addEventListener('click', async function(e) {
        e.preventDefault();

        if (!uploadedPdfFile) {
            showResult('No PDF file uploaded. Please upload a file first.', 'error');
            return;
        }

        try {
            showResult('Analyzing PDF for unwanted elements...', 'loading');
            analyzeBtn.disabled = true;
            analyzeBtn.textContent = 'Analyzing...';

            const formData = new FormData();
            formData.append('pdf', uploadedPdfFile);

            const response = await fetch('/api/pdf/analyze-unwanted-elements', {
                method: 'POST',
                body: formData
            });

            if (response.ok) {
                const analysisResult = await response.json();
                displayAnalysisResults(analysisResult);
                showResult('Analysis complete! Review the detected unwanted elements below.', 'success');
            } else {
                const errorText = await response.text();
                throw new Error(errorText || 'Analysis failed');
            }
        } catch (error) {
            console.error('Analysis error:', error);
            showResult('Failed to analyze PDF. Please try again.', 'error');
        } finally {
            analyzeBtn.disabled = false;
            analyzeBtn.textContent = 'Analyze Unwanted Elements';
        }
    });

    function displayAnalysisResults(analysis) {
        analysisContent.innerHTML = '';

        // Display summary
        const summary = document.createElement('div');
        summary.className = 'analysis-summary';
        summary.innerHTML = `
            <p><strong>Total Pages:</strong> ${analysis.total_pages}</p>
            <p><strong>Image Candidates:</strong> ${analysis.image_candidates.length}</p>
            <p><strong>Text Candidates:</strong> ${analysis.text_candidates.length}</p>
            <p><strong>Overall Confidence:</strong> ${(analysis.overall_confidence * 100).toFixed(1)}%</p>
        `;
        analysisContent.appendChild(summary);

        // Display debug logs if available
        if (analysis.debug_logs && analysis.debug_logs.length > 0) {
            const debugSection = document.createElement('div');
            debugSection.className = 'debug-logs';
            debugSection.innerHTML = `
                <h4>Debug Information</h4>
                <button id="toggleDebugBtn" style="margin-bottom: 10px;">Show Debug Logs</button>
                <div id="debugLogsContent" style="display: none; max-height: 400px; overflow-y: auto; background: #f5f5f5; padding: 10px; border-radius: 4px; font-family: monospace; font-size: 12px; white-space: pre-wrap; word-wrap: break-word;">
                    ${analysis.debug_logs.join('\n')}
                </div>
            `;
            analysisContent.appendChild(debugSection);

            // Toggle debug logs
            document.getElementById('toggleDebugBtn').addEventListener('click', function() {
                const debugContent = document.getElementById('debugLogsContent');
                const btn = document.getElementById('toggleDebugBtn');
                if (debugContent.style.display === 'none') {
                    debugContent.style.display = 'block';
                    btn.textContent = 'Hide Debug Logs';
                } else {
                    debugContent.style.display = 'none';
                    btn.textContent = 'Show Debug Logs';
                }
            });
        }

        // Display recommendations if any
        if (analysis.recommendations && analysis.recommendations.length > 0) {
            const recommendationsDiv = document.createElement('div');
            recommendationsDiv.className = 'recommendations';
            recommendationsDiv.innerHTML = '<strong>Recommendations:</strong><ul>' +
                analysis.recommendations.map(rec => `<li>${rec}</li>`).join('') +
                '</ul>';
            analysisContent.appendChild(recommendationsDiv);
        }

        // Display image candidates
        if (analysis.image_candidates.length > 0) {
            const imageHeader = document.createElement('h4');
            imageHeader.textContent = 'Detected Images (Potential Unwanted Elements):';
            analysisContent.appendChild(imageHeader);

            analysis.image_candidates.forEach(candidate => {
                const candidateDiv = createCandidateElement(candidate, 'image');
                analysisContent.appendChild(candidateDiv);
            });
        }

        // Display text candidates
        if (analysis.text_candidates.length > 0) {
            const textHeader = document.createElement('h4');
            textHeader.textContent = 'Detected Text (Potential Unwanted Elements):';
            analysisContent.appendChild(textHeader);

            analysis.text_candidates.forEach(candidate => {
                const candidateDiv = createCandidateElement(candidate, 'text');
                analysisContent.appendChild(candidateDiv);
            });
        }

        // Show checkboxes if there are candidates
        const totalCandidates = analysis.image_candidates.length + analysis.text_candidates.length;
        if (totalCandidates > 0) {
            elementSelection.style.display = 'block';
            // Populate checkboxes
            populateElementCheckboxes(analysis);
        }

        analysisResults.style.display = 'block';
    }
    
    function populateElementCheckboxes(analysis) {
        elementCheckboxes.innerHTML = '';
        
        const allCandidates = [...analysis.image_candidates, ...analysis.text_candidates];
        allCandidates.forEach(candidate => {
            const checkboxDiv = document.createElement('div');
            checkboxDiv.className = 'checkbox-container';
            
            const checkbox = document.createElement('input');
            checkbox.type = 'checkbox';
            checkbox.id = `elem_${candidate.id}`;
            checkbox.name = 'elements';
            checkbox.value = candidate.id;
            
            const label = document.createElement('label');
            label.htmlFor = `elem_${candidate.id}`;
            label.textContent = candidate.description;
            
            checkboxDiv.appendChild(checkbox);
            checkboxDiv.appendChild(label);
            elementCheckboxes.appendChild(checkboxDiv);
        });
    }
    
    // Handle selective removal form submission
    const selectiveRemovalForm = document.getElementById('selectiveRemovalForm');
    if (selectiveRemovalForm) {
        selectiveRemovalForm.addEventListener('submit', async function(e) {
            e.preventDefault();
            
            if (!uploadedPdfFile) {
                showResult('No PDF file uploaded. Please upload a file first.', 'error');
                return;
            }
            
            const selectedElements = [];
            elementCheckboxes.querySelectorAll('input[type="checkbox"]:checked').forEach(cb => {
                selectedElements.push(cb.value);
            });
            
            if (selectedElements.length === 0) {
                showResult('Please select at least one element to remove.', 'error');
                return;
            }
            
            try {
                showResult('Removing selected elements...', 'loading');
                
                const formData = new FormData();
                formData.append('pdf', uploadedPdfFile);
                formData.append('elements', selectedElements.join(','));
                
                const response = await fetch('/api/pdf/remove-selected-elements', {
                    method: 'POST',
                    body: formData
                });
                
                if (response.ok) {
                    // Get the blob and trigger download
                    const blob = await response.blob();
                    const url = window.URL.createObjectURL(blob);
                    const a = document.createElement('a');
                    a.style.display = 'none';
                    a.href = url;
                    
                    // Get filename from Content-Disposition header if available
                    const contentDisposition = response.headers.get('Content-Disposition');
                    let filename = 'unwanted_elements_removed.pdf';
                    if (contentDisposition) {
                        const filenameMatch = contentDisposition.match(/filename[^;=\n]*=((['"]).*?\2|[^;\n]*)/);
                        if (filenameMatch && filenameMatch[1]) {
                            filename = filenameMatch[1].replace(/['"]/g, '');
                        }
                    }
                    
                    a.download = filename;
                    document.body.appendChild(a);
                    a.click();
                    window.URL.revokeObjectURL(url);
                    a.remove();
                    
                    showResult('Elements removed successfully! Download should start automatically.', 'success');
                } else {
                    const errorText = await response.text();
                    throw new Error(errorText || 'Removal failed');
                }
            } catch (error) {
                console.error('Removal error:', error);
                showResult('Failed to remove elements: ' + error.message, 'error');
            }
        });
    }

    function createCandidateElement(candidate, type) {
        const candidateDiv = document.createElement('div');
        candidateDiv.className = 'unwanted-element-candidate';
        candidateDiv.setAttribute('data-id', candidate.id);

        const header = document.createElement('div');
        header.className = 'candidate-header';

        const typeSpan = document.createElement('span');
        typeSpan.className = `candidate-type ${type}`;
        typeSpan.textContent = candidate.type;

        const confidenceSpan = document.createElement('span');
        confidenceSpan.className = `confidence-score ${getConfidenceClass(candidate.confidence)}`;
        confidenceSpan.textContent = `${(candidate.confidence * 100).toFixed(1)}% confidence`;

        header.appendChild(typeSpan);
        header.appendChild(confidenceSpan);

        const description = document.createElement('div');
        description.innerHTML = `<strong>Description:</strong> ${candidate.description}`;

        const checkboxContainer = document.createElement('div');
        checkboxContainer.className = 'checkbox-container';

        const checkbox = document.createElement('input');
        checkbox.type = 'checkbox';
        checkbox.value = candidate.id;
        checkbox.name = 'elements';
        checkbox.id = `checkbox-${candidate.id}`;

        const label = document.createElement('label');
        label.htmlFor = `checkbox-${candidate.id}`;
        label.textContent = 'Select for removal';

        checkboxContainer.appendChild(checkbox);
        checkboxContainer.appendChild(label);

        candidateDiv.appendChild(header);
        candidateDiv.appendChild(description);
        candidateDiv.appendChild(checkboxContainer);

        return candidateDiv;
    }

    function getConfidenceClass(confidence) {
        if (confidence >= 0.8) return 'high';
        if (confidence >= 0.5) return 'medium';
        return 'low';
    }

    function showResult(message, type) {
        resultContent.className = type;
        resultContent.textContent = message;
        resultDiv.style.display = 'block';

        if (type === 'success' || type === 'error') {
            setTimeout(() => {
                resultDiv.style.display = 'none';
            }, 5000);
        }
    }
});
