// Copy for aio's login, signup and hand-off pages. English is aio's own
// wording; other languages are used only when an app sends users here and
// asks for one with ui_locales (see AuthRequestInfo.locale).

export type Locale = 'en' | 'id';

export interface AuthMessages {
	// shared
	securedBy: string;
	securedByDetail: (app: string) => string;
	username: string;
	password: string;
	// login
	loginTitle: string;
	loginTitleApp: (app: string) => string;
	loginSubtitle: string;
	loginSubtitleApp: string;
	usernamePlaceholder: string;
	passwordPlaceholder: string;
	loginButton: string;
	loggingIn: string;
	signUp: string;
	noAccount: string;
	credentialsRequired: string;
	invalidCredentials: string;
	accountBlocked: string;
	tooManyAttempts: string;
	loginFailed: string;
	loginError: string;
	// two-factor
	twoFactorTitle: string;
	enterRecoveryCode: string;
	enterTotpCode: string;
	recoveryCode: string;
	verificationCode: string;
	verify: string;
	verifying: string;
	useRecoveryCode: string;
	useAuthenticator: string;
	backToLogin: string;
	recoveryCodeRequired: string;
	verificationCodeRequired: string;
	tooManyCodeAttempts: string;
	invalidCode: string;
	// signup
	signupTitle: string;
	signupTitleApp: (app: string) => string;
	signupSubtitle: string;
	signupSubtitleApp: (app: string) => string;
	logIn: string;
	email: string;
	optional: string;
	chooseUsername: string;
	choosePassword: string;
	confirmPassword: string;
	reenterPassword: string;
	signUpButton: string;
	creatingAccount: string;
	haveAccount: string;
	usernamePasswordRequired: string;
	passwordsDontMatch: string;
	passwordTooShort: string;
	usernameTaken: string;
	tooManySignups: string;
	signupFailed: string;
	signupError: string;
	// hand-off (/oauth/login)
	handoffTitle: (app: string) => string;
	handoffDescription: (app: string) => string;
	handoffFallbackTitle: string;
	checkingSession: string;
	continuingTo: (app: string) => string;
	missingRequest: string;
	invalidLink: string;
	couldNotFinish: string;
	useAnotherAccount: string;
	goBack: string;
}

const en: AuthMessages = {
	securedBy: 'Login secured by All-in-one',
	securedByDetail: (app) => `One account for ${app} and other apps that use All-in-one.`,
	username: 'Username',
	password: 'Password',
	loginTitle: 'Login to your account',
	loginTitleApp: (app) => `Log in to ${app}`,
	loginSubtitle: 'Enter your email below to login to your account',
	loginSubtitleApp: 'Use your All-in-one account.',
	usernamePlaceholder: 'Enter your username',
	passwordPlaceholder: 'Your password',
	loginButton: 'Login',
	loggingIn: 'Logging in...',
	signUp: 'Sign Up',
	noAccount: "Don't have an account?",
	credentialsRequired: 'Username and password are required',
	invalidCredentials: 'Invalid username or password.',
	accountBlocked: 'This account is blocked.',
	tooManyAttempts: 'Too many attempts. Please try again later.',
	loginFailed: 'Login failed. Please check your credentials.',
	loginError: 'An error occurred during login. Please try again.',
	twoFactorTitle: 'Two-Factor Authentication',
	enterRecoveryCode: 'Enter one of your recovery codes',
	enterTotpCode: 'Enter the 6-digit code from your authenticator app',
	recoveryCode: 'Recovery Code',
	verificationCode: 'Verification Code',
	verify: 'Verify',
	verifying: 'Verifying...',
	useRecoveryCode: 'Use a recovery code',
	useAuthenticator: 'Use authenticator app instead',
	backToLogin: 'Back to login',
	recoveryCodeRequired: 'Recovery code is required',
	verificationCodeRequired: 'Verification code is required',
	tooManyCodeAttempts: 'Too many attempts. Please login again.',
	invalidCode: 'Invalid code. Please try again.',
	signupTitle: 'Create your account',
	signupTitleApp: (app) => `Create an account for ${app}`,
	signupSubtitle: 'Enter a username and password to get started',
	signupSubtitleApp: (app) => `It's an All-in-one account: you'll use it to log in to ${app}.`,
	logIn: 'Log In',
	email: 'Email',
	optional: '(optional)',
	chooseUsername: 'Choose a username',
	choosePassword: 'Choose a password',
	confirmPassword: 'Confirm Password',
	reenterPassword: 'Re-enter your password',
	signUpButton: 'Sign Up',
	creatingAccount: 'Creating account...',
	haveAccount: 'Already have an account?',
	usernamePasswordRequired: 'Username and password are required',
	passwordsDontMatch: 'Passwords do not match',
	passwordTooShort: 'Password must be at least 3 characters long',
	usernameTaken: 'Username already taken',
	tooManySignups: 'Too many sign-up attempts. Please try again later.',
	signupFailed: 'Failed to create account. Please try again.',
	signupError: 'An error occurred during sign up. Please try again.',
	handoffTitle: (app) => `Continue to ${app}`,
	handoffDescription: (app) => `${app} uses your all-in-one account to log you in.`,
	handoffFallbackTitle: 'Log in with All-in-one',
	checkingSession: 'Checking your all-in-one session…',
	continuingTo: (app) => `Continuing to ${app}…`,
	missingRequest: 'This login link is missing its request. Go back to the app and try again.',
	invalidLink: 'This login link is not valid.',
	couldNotFinish: 'Could not finish logging in.',
	useAnotherAccount: 'Use another account',
	goBack: 'Go back'
};

const id: AuthMessages = {
	securedBy: 'Login diamankan oleh All-in-one',
	securedByDetail: (app) => `Satu akun untuk ${app} dan aplikasi lain yang memakai All-in-one.`,
	username: 'Nama pengguna',
	password: 'Kata sandi',
	loginTitle: 'Masuk ke akun kamu',
	loginTitleApp: (app) => `Masuk ke ${app}`,
	loginSubtitle: 'Masukkan nama pengguna dan kata sandi kamu',
	loginSubtitleApp: 'Pakai akun All-in-one kamu.',
	usernamePlaceholder: 'Masukkan nama pengguna',
	passwordPlaceholder: 'Kata sandi kamu',
	loginButton: 'Masuk',
	loggingIn: 'Sedang masuk...',
	signUp: 'Daftar',
	noAccount: 'Belum punya akun?',
	credentialsRequired: 'Nama pengguna dan kata sandi wajib diisi.',
	invalidCredentials: 'Nama pengguna atau kata sandi salah.',
	accountBlocked: 'Akun ini diblokir.',
	tooManyAttempts: 'Terlalu banyak percobaan. Coba lagi nanti.',
	loginFailed: 'Gagal masuk. Periksa nama pengguna dan kata sandi kamu.',
	loginError: 'Terjadi kesalahan saat masuk. Coba lagi.',
	twoFactorTitle: 'Verifikasi dua langkah',
	enterRecoveryCode: 'Masukkan salah satu kode pemulihan kamu',
	enterTotpCode: 'Masukkan kode 6 digit dari aplikasi autentikator',
	recoveryCode: 'Kode pemulihan',
	verificationCode: 'Kode verifikasi',
	verify: 'Verifikasi',
	verifying: 'Memverifikasi...',
	useRecoveryCode: 'Pakai kode pemulihan',
	useAuthenticator: 'Pakai aplikasi autentikator',
	backToLogin: 'Kembali ke halaman masuk',
	recoveryCodeRequired: 'Kode pemulihan wajib diisi.',
	verificationCodeRequired: 'Kode verifikasi wajib diisi.',
	tooManyCodeAttempts: 'Terlalu banyak percobaan. Silakan masuk lagi.',
	invalidCode: 'Kode salah. Coba lagi.',
	signupTitle: 'Buat akun',
	signupTitleApp: (app) => `Buat akun untuk ${app}`,
	signupSubtitle: 'Pilih nama pengguna dan kata sandi untuk mulai',
	signupSubtitleApp: (app) => `Ini akun All-in-one: kamu memakainya untuk masuk ke ${app}.`,
	logIn: 'Masuk',
	email: 'Email',
	optional: '(opsional)',
	chooseUsername: 'Pilih nama pengguna',
	choosePassword: 'Pilih kata sandi',
	confirmPassword: 'Ulangi kata sandi',
	reenterPassword: 'Ketik ulang kata sandi',
	signUpButton: 'Daftar',
	creatingAccount: 'Membuat akun...',
	haveAccount: 'Sudah punya akun?',
	usernamePasswordRequired: 'Nama pengguna dan kata sandi wajib diisi.',
	passwordsDontMatch: 'Kata sandi tidak sama.',
	passwordTooShort: 'Kata sandi minimal 3 karakter.',
	usernameTaken: 'Nama pengguna sudah dipakai.',
	tooManySignups: 'Terlalu banyak percobaan daftar. Coba lagi nanti.',
	signupFailed: 'Gagal membuat akun. Coba lagi.',
	signupError: 'Terjadi kesalahan saat mendaftar. Coba lagi.',
	handoffTitle: (app) => `Lanjut ke ${app}`,
	handoffDescription: (app) => `${app} memakai akun All-in-one kamu untuk masuk.`,
	handoffFallbackTitle: 'Masuk dengan All-in-one',
	checkingSession: 'Memeriksa sesi All-in-one kamu…',
	continuingTo: (app) => `Melanjutkan ke ${app}…`,
	missingRequest: 'Tautan masuk ini tidak lengkap. Kembali ke aplikasi dan coba lagi.',
	invalidLink: 'Tautan masuk ini tidak valid.',
	couldNotFinish: 'Gagal menyelesaikan proses masuk.',
	useAnotherAccount: 'Pakai akun lain',
	goBack: 'Kembali'
};

const catalogs: Record<Locale, AuthMessages> = { en, id };

export function authMessages(locale: string | undefined | null): AuthMessages {
	return catalogs[(locale as Locale) in catalogs ? (locale as Locale) : 'en'];
}
