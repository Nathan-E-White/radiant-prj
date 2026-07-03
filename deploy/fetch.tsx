import React, { useState } from 'react';

// Define types matching our Go API struct shapes
interface JobSubmitRequest {
    node_count: number;
    partition: 'cpu-short' | 'cpu-long' | 'gpu';
    script_name: string;
}

interface JobResponse {
    message?: string;
    job_id?: string;
    error?: string;
}

export const SlurmJobSubmitter: React.FC = () => {
    // Form State
    const [nodeCount, setNodeCount] = useState<number>(1);
    const [partition, setPartition] = useState<JobSubmitRequest['partition']>('cpu-short');
    const [scriptName, setScriptName] = useState<string>('');

    // UI Feedback States
    const [isLoading, setIsLoading] = useState<boolean>(false);
    const [successMessage, setSuccessMessage] = useState<string | null>(null);
    const [errorMessage, setErrorMessage] = useState<string | null>(null);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();

        // Reset previous execution alerts
        setIsLoading(true);
        setSuccessMessage(null);
        setErrorMessage(null);

        const payload: JobSubmitRequest = {
            node_count: nodeCount,
            partition: partition,
            script_name: scriptName.trim(),
        };

        try {
            const response = await fetch('http://localhost:8080/api/jobs/submit', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    // If you add JWT authentication later, include it here:
                    // 'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify(payload),
            });

            const data: JobResponse = await response.json();

            if (!response.ok) {
                // Captures 400 (Bad JSON), 422 (Validation failed), and 500 (Slurm errors)
                throw new Error(data.error || `HTTP error! Status: ${response.status}`);
            }

            // Success branch
            setSuccessMessage(`${data.message}. Cluster Output: ${data.job_id}`);
            setScriptName(''); // Clear input on successful queue
        } catch (err: any) {
            setErrorMessage(err.message || 'An unexpected connection error occurred.');
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <div style={{ maxWidth: '400px', margin: '20px auto', fontFamily: 'sans-serif' }}>
            <h2>Submit Cluster Job</h2>

            <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '15px' }}>
                <div>
                    <label style={{ display: 'block', marginBottom: '5px' }}>Node Count (1-32):</label>
                    <input
                        type="number"
                        min={1}
                        max={32}
                        value={nodeCount}
                        onChange={(e) => setNodeCount(parseInt(e.target.value, 10) || 1)}
                        style={{ width: '100%', padding: '8px' }}
                        required
                    />
                </div>

                <div>
                    <label style={{ display: 'block', marginBottom: '5px' }}>Partition / Queue:</label>
                    <select
                        value={partition}
                        onChange={(e) => setPartition(e.target.value as JobSubmitRequest['partition'])}
                        style={{ width: '100%', padding: '8px' }}
                    >
                        <option value="cpu-short">CPU Short (Fast Queue)</option>
                        <option value="cpu-long">CPU Long</option>
                        <option value="gpu">GPU Cluster</option>
                    </select>
                </div>

                <div>
                    <label style={{ display: 'block', marginBottom: '5px' }}>Script Name (Alphanumeric only):</label>
                    <input
                        type="text"
                        placeholder="e.g., data-crunch"
                        value={scriptName}
                        onChange={(e) => setScriptName(e.target.value)}
                        style={{ width: '100%', padding: '8px' }}
                        required
                    />
                </div>

                <button
                    type="submit"
                    disabled={isLoading}
                    style={{ padding: '10px', backgroundColor: '#0070f3', color: 'white', border: 'none', cursor: 'pointer' }}
                >
                    {isLoading ? 'Queueing Job...' : 'Submit to Slurm'}
                </button>
            </form>

            {/* Visual Anchors for Feedback Statuses */}
            {successMessage && (
                <div style={{ marginTop: '15px', padding: '10px', backgroundColor: '#e6fffa', color: '#006d5b', border: '1px solid #b2f5ea' }}>
                    ✅ {successMessage}
                </div>
            )}

            {errorMessage && (
                <div style={{ marginTop: '15px', padding: '10px', backgroundColor: '#fff5f5', color: '#c53030', border: '1px solid #fed7d7' }}>
                    ❌ Error: {errorMessage}
                </div>
            )}
        </div>
    );
};
