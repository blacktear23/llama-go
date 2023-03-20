export interface PromptRequest {
    prompt: string;
    tokens: number;
    stream: boolean;
}

export interface MessageItem {
    role: string;
    text: string;
    loading: boolean;
}