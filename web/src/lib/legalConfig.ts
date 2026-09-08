export interface LegalConfig {
	schoolName: string;
	payoutDays: string;
	contactEmails: string[];
	controllers: string[];
	controllerEmails: string[];
}

export const legalConfig: LegalConfig = {
	schoolName: 'Gymnázium Christiana Dopplera',
	payoutDays: '5 pracovních dnů',
	contactEmails: ['burza@skrat.org'],
	controllers: ['Ondřej Urban', 'Rostislav Kozlík'],
	controllerEmails: ['urbano@gchd.cz', 'burza@skrat.org']
};
