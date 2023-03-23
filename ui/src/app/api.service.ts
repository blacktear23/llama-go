export interface PromptRequest {
    prompt: string;
    history: string;
    stream: boolean;
    tokens: number|null;
    top_k: number|null;
    top_p: number|null;
    temp: number|null;
    repeat_penalty: number|null;
    repeat_lastn: number|null;
}

export interface MessageItem {
    role: string;
    text: string;
    loading: boolean;
}